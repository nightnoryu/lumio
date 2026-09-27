package httptransport

import "testing"

func TestPublicHostBoundary(t *testing.T) {
	for _, host := range []string{"anna.lumio.test", "a-1.lumio.test"} {
		if _, ok := publicSlug(host, "lumio.test", ""); !ok {
			t.Errorf("rejected %s", host)
		}
	}
	for _, host := range []string{"lumio.test", "app.lumio.test", "anna.lumio.test.evil.test", "anna.evillumio.test", "anna.other.test", "one.two.lumio.test", "Anna.lumio.test", "anna.lumio.test.", "anna.lumio.test:444", "anna@lumio.test", "-anna.lumio.test", "xn--abc.lumio.test", "anna%2elumio.test"} {
		if _, ok := publicSlug(host, "lumio.test", ""); ok {
			t.Errorf("accepted %s", host)
		}
	}
	if slug, ok := publicSlug("anna.localhost:3000", "localhost", "3000"); !ok || slug != "anna" {
		t.Fatal("local preview authority rejected")
	}
	if _, ok := publicSlug("anna.localhost:4000", "localhost", "3000"); ok {
		t.Fatal("wrong port accepted")
	}
}
