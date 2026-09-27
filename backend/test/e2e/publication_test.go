//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"lumio/internal/app"
	"lumio/internal/domain"
	"lumio/internal/infrastructure/postgres"
)

type publicObjects struct{ app.ObjectStore }

func (publicObjects) Open(_ context.Context, key string) (io.ReadCloser, int64, error) {
	if !strings.HasSuffix(key, "/960.jpg") {
		return nil, 0, domain.ErrNotFound
	}
	return io.NopCloser(strings.NewReader("optimized photograph")), 20, nil
}
func (f *identityFixture) public(host, path string, status int, tags ...string) *testResponse {
	f.t.Helper()
	req, err := http.NewRequestWithContext(f.t.Context(), "GET", f.server.URL+path, http.NoBody)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Host = host
	if len(tags) > 0 {
		req.Header.Set("If-None-Match", tags[0])
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	if response.StatusCode != status {
		f.t.Fatalf("%s%s: got %d want %d: %s", host, path, response.StatusCode, status, body)
	}
	return &testResponse{header: response.Header, body: body}
}
func addPublishPhoto(t *testing.T, f *identityFixture, site string) string {
	t.Helper()
	id := uuid.NewString()
	raw, _ := json.Marshal([]string{"media/" + id + "/v1/attempt/960.jpg"})
	_, err := f.db.TransactionalClient().ExecContext(t.Context(), `INSERT INTO photo(id,site_id,name,size,content_type,status,original,expires_at,width,height,lease,variants) VALUES($1,$2,'portrait.jpg',100,'image/jpeg','ready',$3,now(),1200,800,'attempt',$4)`, id, site, "media/"+id+"/original", string(raw))
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestPublicationJourney(t *testing.T) {
	f := newIdentityFixture(t, publicObjects{})
	token := f.register("publisher@example.com")
	other := f.register("other-publisher@example.com")
	owner, err := f.service.Authenticate(t.Context(), token)
	if err != nil {
		t.Fatal(err)
	}
	site, err := f.service.CreateSite(t.Context(), owner.ID, "anna")
	if err != nil {
		t.Fatal(err)
	}
	another, err := f.service.CreateSite(t.Context(), owner.ID, "other-site")
	if err != nil {
		t.Fatal(err)
	}
	photo := addPublishPhoto(t, f, site.ID)
	unused := addPublishPhoto(t, f, site.ID)
	foreign := addPublishPhoto(t, f, another.ID)
	draft := domain.EmptyDraft()
	draft.DisplayName = "Anna Photographer"
	draft.Biography = "Portraits in natural light"
	draft.SEOTitle = "Anna — Portraits"
	draft.SEODescription = "Beautiful portraits in Berlin"
	draft.Photos = []domain.DraftPhoto{{ID: photo, Alt: "Portrait in sunlight"}}
	draft.Contacts = []domain.Contact{{Label: "Email", URL: "mailto:anna@example.com"}}
	path := "/api/sites/" + site.ID
	save := func() {
		t.Helper()
		raw, _ := json.Marshal(draft)
		response := f.call("PUT", path+"/draft", string(raw), token, testOrigin, app.CSRF(token), 200)
		if decodeErr := json.Unmarshal(response.body, &draft); decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	publish := func(version int64, status int) {
		t.Helper()
		f.call("POST", path+"/publication", `{"version":`+strconv.FormatInt(version, 10)+`}`, token, testOrigin, app.CSRF(token), status)
	}
	f.public("anna.lumio.test", "/", 404)
	save()
	f.call("POST", path+"/publication", `{"version":1}`, other, testOrigin, app.CSRF(other), 404)
	f.call("DELETE", path+"/publication", "", other, testOrigin, app.CSRF(other), 404)
	f.call("GET", path+"/publication", "", other, "", "", 404)
	f.call("POST", path+"/publication", `{"version":1}`, token, testOrigin, "", 403)
	publish(draft.Version+1, 409)
	// Ready state is checked again at publication, independently of draft validation.
	if _, err = f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE photo SET status='processing' WHERE id=$1`, photo); err != nil {
		t.Fatal(err)
	}
	publish(draft.Version, 400)
	if _, err = f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE photo SET status='ready' WHERE id=$1`, photo); err != nil {
		t.Fatal(err)
	}
	publish(draft.Version, 200)
	before := f.public("anna.lumio.test", "/", 200)
	checkPublicHTML(t, before)
	publish(draft.Version, 200)
	if again := f.public("anna.lumio.test", "/", 200); !bytes.Equal(before.body, again.body) {
		t.Fatal("repeated publication changed revision")
	}
	match := regexp.MustCompile(`/images/[a-f0-9-]+/[a-f0-9-]+/0`).Find(before.body)
	if len(match) == 0 {
		t.Fatal("missing optimized image URL")
	}
	imagePath := string(match)
	if image := f.public("anna.lumio.test", imagePath, 200); string(image.body) != "optimized photograph" || image.header.Get("Content-Type") != "image/jpeg" {
		t.Fatal("image delivery failed")
	}
	image := f.public("anna.lumio.test", imagePath, 200)
	if image.header.Get("Cache-Control") != "private, no-cache" || image.header.Get("ETag") == "" {
		t.Fatal("image cache policy missing")
	}
	tag := image.header.Get("ETag")
	f.public("anna.lumio.test", imagePath, 304, tag)
	for _, bad := range []string{strings.Replace(imagePath, photo, unused, 1), strings.Replace(imagePath, photo, foreign, 1), imagePath + "/original", strings.TrimSuffix(imagePath, "0") + "99", "/media/" + photo + "/original", "/api/sites", "/preview/" + site.ID} {
		f.public("anna.lumio.test", bad, 404)
	}
	for _, host := range []string{"unknown.lumio.test", "other-site.lumio.test", "anna.lumio.test.evil.test", "sub.anna.lumio.test", "anna.other.test", "anna.lumio.test.", "anna.lumio.test:444"} {
		f.public(host, "/", 404)
	}
	draft.DisplayName = "Unpublished new name"
	draft.SEOTitle = "Unpublished SEO"
	draft.Photos = []domain.DraftPhoto{{ID: unused, Alt: "Another portrait"}}
	save()
	if after := f.public("anna.lumio.test", "/", 200); !bytes.Equal(before.body, after.body) {
		t.Fatal("draft changes affected live site")
	}
	f.call("DELETE", path+"/photos/"+photo, "", token, testOrigin, app.CSRF(token), 400)
	f.public("anna.lumio.test", imagePath, 200)
	f.call("DELETE", path+"/publication", "", token, testOrigin, "", 403)
	f.call("DELETE", path+"/publication", "", token, testOrigin, app.CSRF(token), 204)
	f.public("anna.lumio.test", "/", 404)
	f.public("anna.lumio.test", imagePath, 404, tag)
	f.call("DELETE", path+"/photos/"+photo, "", token, testOrigin, app.CSRF(token), 204)
	publish(draft.Version, 200)
	after := f.public("anna.lumio.test", "/", 200)
	if !strings.Contains(string(after.body), "Unpublished new name") {
		t.Fatal("new revision not activated")
	}
	f.public("anna.lumio.test", imagePath, 404, tag)
	store := &postgres.Store{DB: f.db.TransactionalClient()}
	if _, err = store.CleanupPhoto(t.Context()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unexpected cleanup: %v", err)
	}
}
func checkPublicHTML(t *testing.T, response *testResponse) {
	t.Helper()
	for _, text := range []string{"Anna Photographer", "Portraits in natural light", "<title>Anna — Portraits</title>", `name="description" content="Beautiful portraits in Berlin"`, `rel="canonical" href="https://anna.lumio.test/"`, `property="og:url" content="https://anna.lumio.test/"`, `property="og:image" content="https://anna.lumio.test/images/`, `alt="Portrait in sunlight"`, `<dialog`, `srcset="/images/`, `960w"`, `sizes="`, `loading="eager"`} {
		if !strings.Contains(string(response.body), text) {
			t.Errorf("missing %s", text)
		}
	}
	if strings.Contains(string(response.body), "noindex") || response.header.Get("X-Robots-Tag") != "" || response.header.Get("Set-Cookie") != "" {
		t.Fatal("public page has private headers")
	}
}
