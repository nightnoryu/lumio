//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"lumio/internal/app"
	"lumio/internal/domain"
	"lumio/internal/infrastructure/postgres"
)

func TestPortfolioDraftIsolation(t *testing.T) {
	f := newIdentityFixture(t, previewObjects{})
	owner := f.register("editor@example.com")
	other := f.register("other-editor@example.com")
	response := f.call("POST", "/api/sites", `{"slug":"editor"}`, owner, testOrigin, app.CSRF(owner), 200)
	var site struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.body, &site); err != nil {
		t.Fatal(err)
	}
	path := "/api/sites/" + site.ID + "/draft"
	response = f.call("GET", path, "", owner, "", "", 200)
	var draft domain.Draft
	if err := json.Unmarshal(response.body, &draft); err != nil {
		t.Fatal(err)
	}
	draft.DisplayName = "Anna"
	draft.Biography = "Portrait photographer"
	draft.Contacts = []domain.Contact{{Label: "Email", URL: "mailto:anna@example.com"}}
	raw, _ := json.Marshal(draft)
	f.call("PUT", path, string(raw), other, testOrigin, app.CSRF(other), 404)
	f.call("GET", path, "", other, "", "", 404)
	f.call("PUT", path, string(raw), owner, testOrigin, "", 403)
	response = f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 200)
	f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 409)
	if err := json.Unmarshal(response.body, &draft); err != nil {
		t.Fatal(err)
	}
	if draft.Version != 1 {
		t.Fatal("version did not advance")
	}
	snapshot := response.body
	revision := uuid.NewString()
	if _, err := f.db.TransactionalClient().ExecContext(t.Context(), `INSERT INTO site_revision(id,site_id,document) VALUES($1,$2,$3)`, revision, site.ID, snapshot); err != nil {
		t.Fatal(err)
	}
	draft.DisplayName = "Changed draft"
	raw, _ = json.Marshal(draft)
	f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 200)
	var stored []byte
	if err := f.db.TransactionalClient().GetContext(t.Context(), &stored, `SELECT document FROM site_revision WHERE id=$1`, revision); err != nil {
		t.Fatal(err)
	}
	var published domain.Draft
	if err := json.Unmarshal(stored, &published); err != nil {
		t.Fatal(err)
	}
	if published.DisplayName != "Anna" {
		t.Fatal("saving draft changed revision")
	}
	if _, err := f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE site_revision SET document='{}' WHERE id=$1`, revision); err == nil {
		t.Fatal("revision is mutable")
	}
	draft.Version = 2
	draft.Photos = []domain.DraftPhoto{{ID: uuid.NewString()}}
	raw, _ = json.Marshal(draft)
	f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 400)
	// A ready photograph can be selected, but cannot be deleted while referenced.
	photo := "5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f"
	if _, err := f.db.TransactionalClient().ExecContext(t.Context(), `INSERT INTO photo(id,site_id,name,size,content_type,status,original,watermark,expires_at,variants) VALUES($1,$2,'portrait',100,'image/jpeg','ready','private/original','',now(),'["optimized/portrait.jpg"]')`, photo, site.ID); err != nil {
		t.Fatal(err)
	}
	draft.Photos = []domain.DraftPhoto{}
	for _, reference := range []string{strings.ToUpper(photo), strings.ReplaceAll(photo, "-", "")} {
		draft.ProfilePhoto = reference
		raw, _ = json.Marshal(draft)
		f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 400)
		draft.ProfilePhoto = ""
		draft.CoverPhoto = reference
		raw, _ = json.Marshal(draft)
		f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 400)
		draft.CoverPhoto = ""
	}
	draft.Photos = []domain.DraftPhoto{{ID: photo, Alt: "A portrait"}}
	raw, _ = json.Marshal(draft)
	f.call("PUT", path, string(raw), owner, testOrigin, app.CSRF(owner), 200)
	previewPath := "/preview/" + site.ID
	f.call("GET", previewPath, "", "", "", "", 401)
	f.call("GET", previewPath, "", other, "", "", 404)
	preview := f.call("GET", previewPath, "", owner, "", "", 200)
	if preview.header.Get("Cache-Control") != "no-store" || preview.header.Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatal("preview privacy headers missing")
	}
	if !strings.Contains(string(preview.body), "Changed draft") || !strings.Contains(string(preview.body), "https://images.example/optimized/portrait.jpg") {
		t.Fatal("preview did not render draft and optimized photograph")
	}
	user, err := f.service.Authenticate(t.Context(), owner)
	if err != nil {
		t.Fatal(err)
	}
	store := &postgres.Store{DB: f.db.TransactionalClient()}
	if err = store.DeletePhoto(t.Context(), user.ID, site.ID, photo); err == nil || !strings.Contains(err.Error(), "used by a portfolio") {
		t.Fatalf("photo deletion: %v", err)
	}
}

type previewObjects struct{ app.ObjectStore }

func (previewObjects) PreviewURL(_ context.Context, key string) (string, error) {
	return "https://images.example/" + key, nil
}
