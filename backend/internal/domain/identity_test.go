package domain

import (
	"strings"
	"testing"
)

func TestNormalizeSlug(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{" Anna-Photo ", "anna-photo"}, {"A", "a"}, {"42", "42"}, {strings.Repeat("a", 63), strings.Repeat("a", 63)}} {
		input, want := tc.input, tc.want
		got, err := NormalizeSlug(input)
		if err != nil || got != want {
			t.Errorf("NormalizeSlug(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "-anna", "anna-", "anna.photo", "a_b", "фото", "xn--photo", "APP", "api", "www", "admin", "grafana", "s3", "mail", strings.Repeat("a", 64)} {
		if _, err := NormalizeSlug(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
