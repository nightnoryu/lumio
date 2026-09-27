//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"lumio/api/server/publicapi"
	"lumio/internal/app"
	"lumio/internal/infrastructure/storage"
)

func TestDirectUploads(t *testing.T) {
	if os.Getenv("LUMIO_TEST_STORAGE") != "1" {
		t.Skip("start compose storage-init and set LUMIO_TEST_STORAGE=1")
	}
	objects, err := storage.New(t.Context(), "http://localhost:9000", "http://localhost:9000", "us-east-1", "lumio", "lumio-local", "lumio-local-only")
	if err != nil {
		t.Fatal(err)
	}
	f := newIdentityFixture(t, objects)
	token := f.register("upload@example.com")
	other := f.register("other-upload@example.com")
	response := f.call("POST", "/api/sites", `{"slug":"upload"}`, token, testOrigin, app.CSRF(token), 200)
	var site publicapi.Site
	if err = json.Unmarshal(response.body, &site); err != nil {
		t.Fatal(err)
	}
	path := "/api/sites/" + site.ID + "/photos"
	body := `{"name":"photo.jpg","size":4,"contentType":"image/jpeg","watermark":""}`
	f.call("POST", path, body, token, testOrigin, "", 403)
	f.call("POST", path, body, other, testOrigin, app.CSRF(other), 404)
	f.call("POST", path, `{"name":"huge.jpg","size":52428801,"contentType":"image/jpeg","watermark":""}`, token, testOrigin, app.CSRF(token), 400)
	response = f.call("POST", path, body, token, testOrigin, app.CSRF(token), 200)
	var upload publicapi.Upload
	if err = json.Unmarshal(response.body, &upload); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if cleanupErr := objects.DeletePrefix(ctx, "uploads/"+upload.ID+"/"); cleanupErr != nil {
			t.Error(cleanupErr)
		}
		if cleanupErr := objects.DeletePrefix(ctx, "media/"+upload.ID+"/"); cleanupErr != nil {
			t.Error(cleanupErr)
		}
	})
	send := func(address string, data []byte, conditional bool, want int) {
		t.Helper()
		req, requestErr := http.NewRequestWithContext(t.Context(), "PUT", address, bytes.NewReader(data))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		req.Header.Set("Content-Type", "image/jpeg")
		if conditional {
			req.Header.Set("If-None-Match", "*")
		}
		res, requestErr := http.DefaultClient.Do(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != want && (want != 403 || res.StatusCode != 400) {
			t.Fatalf("PUT got %d want %d: %s", res.StatusCode, want, raw)
		}
	}
	send(upload.URL, []byte("12345"), true, 403)
	send(upload.URL, []byte("1234"), false, 403)
	send(upload.URL, []byte("1234"), true, 200)
	send(upload.URL, []byte("4321"), true, 412)
	unsigned, err := url.Parse(upload.URL)
	if err != nil {
		t.Fatal(err)
	}
	unsigned.RawQuery = ""
	req, err := http.NewRequestWithContext(t.Context(), "GET", unsigned.String(), http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := res.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if res.StatusCode != 403 {
		t.Fatal("anonymous original access", res.StatusCode)
	}
	f.call("POST", path+"/"+upload.ID+"/complete", "", other, testOrigin, app.CSRF(other), 404)
	f.call("POST", path+"/"+upload.ID+"/complete", "", token, testOrigin, app.CSRF(token), 204)
	f.call("POST", path+"/"+upload.ID+"/complete", "", token, testOrigin, app.CSRF(token), 204)
	if err = objects.DeletePrefix(t.Context(), "uploads/"+upload.ID+"/"); err != nil {
		t.Fatal(err)
	}
	var preserved bytes.Buffer
	if err = objects.Download(t.Context(), "media/"+upload.ID+"/original", &preserved, 4); err != nil || preserved.String() != "1234" {
		t.Fatal("accepted original lost", err)
	}
	f.call("GET", path+"/"+upload.ID+"/preview", "", token, "", "", 409)
	f.call("DELETE", path+"/"+upload.ID, "", other, testOrigin, app.CSRF(other), 404)
	f.call("DELETE", path+"/"+upload.ID, "", token, testOrigin, app.CSRF(token), 204)
}
