//go:build e2e

package e2e

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"lumio/api/server/publicapi"
	"lumio/internal/app"
	"lumio/internal/domain"
	"lumio/internal/infrastructure/postgres"
)

func TestIdentityJourney(t *testing.T) {
	f := newIdentityFixture(t)
	alice := f.register("alice@example.com")
	bob := f.register("bob@example.com")
	f.call("GET", "/api/auth/me", "", "", "", "", 401)
	f.call("GET", "/api/auth/me", "", app.Token(), "", "", 401)
	meResponse := f.call("GET", "/api/auth/me", "", alice, "", "", 200)
	var viewer publicapi.Viewer
	if err := json.Unmarshal(meResponse.body, &viewer); err != nil {
		t.Fatal(err)
	}
	if viewer.Email != "alice@example.com" || viewer.CsrfToken != app.CSRF(alice) {
		t.Fatal("incorrect viewer")
	}
	for _, origin := range []string{"", "null", "https://anna.lumio.test", "https://app.lumio.test.evil"} {
		f.call("POST", "/api/sites", `{"slug":"alice"}`, alice, origin, app.CSRF(alice), 403)
		f.call("POST", "/api/auth/login", `{"email":"alice@example.com","password":"long-enough-password"}`, "", origin, "", 403)
	}
	for _, csrf := range []string{"", app.CSRF(bob)} {
		f.call("POST", "/api/sites", `{"slug":"alice"}`, alice, testOrigin, csrf, 403)
	}
	f.call("POST", "/api/sites", `{"slug":"Grafana"}`, alice, testOrigin, app.CSRF(alice), 400)
	response := f.call("POST", "/api/sites", `{"slug":" Alice-Photo "}`, alice, testOrigin, app.CSRF(alice), 200)
	var site publicapi.Site
	if err := json.Unmarshal(response.body, &site); err != nil {
		t.Fatal(err)
	}
	if site.Slug != "alice-photo" {
		t.Fatal("slug not normalized")
	}
	f.call("POST", "/api/sites", `{"slug":"alice-photo"}`, bob, testOrigin, app.CSRF(bob), 409)
	f.call("GET", "/api/sites/"+site.ID, "", bob, "", "", 404)
	f.call("PATCH", "/api/sites/"+site.ID, `{"slug":"stolen"}`, bob, testOrigin, app.CSRF(bob), 404)
	f.call("GET", "/api/sites/"+site.ID, "", alice, "", "", 200)
	f.call("PATCH", "/api/sites/"+site.ID, `{"slug":"renamed"}`, alice, testOrigin, app.CSRF(alice), 200)
	f.call("POST", "/api/sites", `{"slug":"second"}`, alice, testOrigin, app.CSRF(alice), 200)
	list := f.call("GET", "/api/sites", "", bob, "", "", 200)
	if strings.TrimSpace(string(list.body)) != "[]" {
		t.Fatal("other owner's sites leaked")
	}
	// Recreate the application adapter to verify the session and site live in PostgreSQL.
	f.service.Store = &postgres.Store{DB: f.db.TransactionalClient()}
	f.call("GET", "/api/sites/"+site.ID, "", alice, "", "", 200)
	logout := f.call("POST", "/api/auth/logout", "", alice, testOrigin, app.CSRF(alice), 204)
	if (&http.Response{Header: logout.header}).Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	f.call("GET", "/api/auth/me", "", alice, "", "", 401)
	if _, err := f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE session SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, app.TokenHash(bob)); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/api/auth/me", "", bob, "", "", 401)
	f.call("POST", "/api/auth/login", `{"email":"alice@example.com","password":"wrong-password"}`, "", testOrigin, "", 401)
	login := f.call("POST", "/api/auth/login", `{"email":"ALICE@example.com","password":"long-enough-password"}`, "", testOrigin, "", 204)
	active := (&http.Response{Header: login.header}).Cookies()[0].Value
	if active == alice {
		t.Fatal("session token reused")
	}
	reset, err := f.service.IssueReset(t.Context(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	resetBody := `{"token":"` + reset + `","password":"replacement-password"}`
	f.call("POST", "/api/auth/reset-password", resetBody, "", testOrigin, "", 204)
	f.call("POST", "/api/auth/reset-password", resetBody, "", testOrigin, "", 401)
	f.call("GET", "/api/auth/me", "", active, "", "", 401)
	f.call("POST", "/api/auth/login", `{"email":"alice@example.com","password":"long-enough-password"}`, "", testOrigin, "", 401)
	f.call("POST", "/api/auth/login", `{"email":"alice@example.com","password":"replacement-password"}`, "", testOrigin, "", 204)
	user, err := f.service.Store.UserByEmail(t.Context(), "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.Store.CreateSession(t.Context(), domain.Session{UserID: user.ID, TokenHash: app.TokenHash(app.Token()), ExpiresAt: time.Now().Add(time.Hour)}, "old-password-hash"); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("stale login accepted:", err)
	}
}
