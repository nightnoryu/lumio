//go:build e2e

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/nightnoryu/go-kita/jsonlog"
	"github.com/nightnoryu/go-kita/postgresql"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"lumio/data/migrations"
	"lumio/internal/app"
	"lumio/internal/domain"
	"lumio/internal/infrastructure/password"
	"lumio/internal/infrastructure/postgres"
	httptransport "lumio/internal/transport/http"
)

const cookieName = "__Host-lumio-session"

const testOrigin = "https://app.lumio.test"
const testPassword = "long-enough-password"

type identityFixture struct {
	t       *testing.T
	db      postgresql.Connector
	service *app.Service
	server  *httptest.Server
}

func newIdentityFixture(t *testing.T, objects ...app.ObjectStore) *identityFixture {
	t.Helper()
	ctx := t.Context()
	databasePassword := rand.Text()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("lumio"), tcpostgres.WithUsername("lumio"),
		tcpostgres.WithPassword(databasePassword), tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if terminateErr := container.Terminate(cleanup); terminateErr != nil {
			t.Error(terminateErr)
		}
	})
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	db := postgresql.NewConnector()
	dsn := postgresql.DSN{Host: host, Port: int(port.Num()), Database: "lumio", User: "lumio", Password: databasePassword}
	if err = db.Open(ctx, dsn, postgresql.Config{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	logger, err := jsonlog.NewLogger(&jsonlog.Config{AppName: "test", Level: jsonlog.ErrorLevel})
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := db.Migrator(logger, migrations.UpFS)
	if err != nil {
		t.Fatal(err)
	}
	if err = migrator.MigrateUp(ctx); err != nil {
		t.Fatal(err)
	}
	if err = migrator.MigrateUp(ctx); err != nil {
		t.Fatal("migration replay:", err)
	}
	service := &app.Service{Store: &postgres.Store{DB: db.TransactionalClient()}, Passwords: password.Argon{}}
	var media *app.Media
	if len(objects) > 0 {
		media = &app.Media{Store: &postgres.Store{DB: db.TransactionalClient()}, Objects: objects[0], Limits: domain.MediaLimits{FileBytes: 52428800, StorageBytes: 2147483648, Photos: 100}}
	}
	handler, err := httptransport.NewRouter(fstest.MapFS{"index.html": {Data: []byte("Lumio")}}, db.Ping, logger, httptransport.APIConfig{Service: service, Media: media, Origin: testOrigin, BaseDomain: "lumio.test"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return &identityFixture{t: t, db: db, service: service, server: server}
}

type testResponse struct {
	header http.Header
	body   []byte
}

func (f *identityFixture) call(method, path, body, token, origin, csrf string, status int) *testResponse {
	f.t.Helper()
	req, err := http.NewRequestWithContext(f.t.Context(), method, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Host = "app.lumio.test"
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: cookieName, Value: token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			f.t.Error(closeErr)
		}
	}()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	if response.StatusCode != status {
		f.t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.StatusCode, status, data)
	}
	return &testResponse{header: response.Header, body: data}
}

func (f *identityFixture) register(email string) string {
	f.t.Helper()
	invitation, err := f.service.Invite(f.t.Context(), email)
	if err != nil {
		f.t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"email": email, "password": testPassword, "invitation": invitation})
	response := f.call("POST", "/api/auth/register", string(body), "", testOrigin, "", 204)
	cookies := (&http.Response{Header: response.header}).Cookies()
	if len(cookies) != 1 {
		f.t.Fatal("missing session cookie")
	}
	cookie := cookies[0]
	if cookie.Name != cookieName || !cookie.Secure || !cookie.HttpOnly || cookie.Domain != "" || cookie.Path != "/" || cookie.SameSite != http.SameSiteStrictMode {
		f.t.Fatalf("unsafe cookie: %+v", cookie)
	}
	f.call("POST", "/api/auth/register", string(body), "", testOrigin, "", 403)
	return cookie.Value
}
