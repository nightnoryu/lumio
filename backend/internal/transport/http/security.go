package httptransport

import (
	"net/http"
	"net/url"
)

func (h *apiHandler) security(next http.Handler) http.Handler {
	storage := ""
	if u, err := url.Parse(h.config.StorageOrigin); err == nil && u.Host != "" && (u.Scheme == "https" || u.Scheme == "http") {
		storage = " " + u.Scheme + "://" + u.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if origin, _ := url.Parse(h.config.Origin); origin != nil && origin.Scheme == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:"+storage+"; connect-src 'self'"+storage+"; font-src 'self'; frame-src 'self'; frame-ancestors 'self'; base-uri 'none'; form-action 'self'; object-src 'none'")
		next.ServeHTTP(w, r)
	})
}
