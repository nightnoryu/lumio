package httptransport

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"lumio/internal/domain"
	"lumio/internal/portfolio"
)

func (h *apiHandler) hostRouter(dashboard http.Handler) http.Handler {
	origin, _ := url.Parse(h.config.Origin)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == healthPath || r.URL.Path == livePath {
			dashboard.ServeHTTP(w, r)
			return
		}
		if h.config.Origin == "" || r.Host == origin.Host {
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			dashboard.ServeHTTP(w, r)
			return
		}
		if r.Host == h.config.BaseDomain && r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			http.Redirect(w, r, h.config.Origin+"/", http.StatusFound)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		slug, valid := publicSlug(r.Host, h.config.BaseDomain, origin.Port())
		if !valid || h.config.Portfolio == nil {
			http.NotFound(w, r)
			return
		}
		h.publicPortfolio(w, r, slug)
	})
}
func viewerScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method != http.MethodHead {
		_, _ = w.Write(portfolio.ViewerJS)
	}
}
func (h *apiHandler) publicPortfolio(w http.ResponseWriter, r *http.Request, slug string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/images/") {
		h.publicImage(w, r, slug)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != viewerPath {
		http.NotFound(w, r)
		return
	}
	revision, err := h.config.Portfolio.Published(r.Context(), slug)
	if err != nil {
		h.publicError(w, r, err)
		return
	}
	if r.URL.Path == viewerPath {
		viewerScript(w, r)
		return
	}
	images := make(map[string]string, len(revision.Images))
	sources := make(map[string]string, len(revision.Images))
	for id, image := range revision.Images {
		width := 0
		for index, variant := range image.Variants {
			if variant.Format == "jpg" {
				if sources[id] != "" {
					sources[id] += ", "
				}
				sources[id] += "/images/" + revision.ID + "/" + id + "/" + strconv.Itoa(index) + " " + strconv.Itoa(variant.Width) + "w"
			}
			if variant.Format == "jpg" && variant.Width > width {
				images[id] = "/images/" + revision.ID + "/" + id + "/" + strconv.Itoa(index)
				width = variant.Width
			}
		}
	}
	var body bytes.Buffer
	err = portfolio.Page(revision.Draft, images, portfolio.Metadata{Canonical: h.publicationURL(slug), Sources: sources}).Render(r.Context(), &body)
	if err != nil {
		h.publicError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method != http.MethodHead {
		_, _ = w.Write(body.Bytes())
	}
}
func (h *apiHandler) publicError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, domain.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	h.logger.Error(err, "public portfolio request failed")
	http.Error(w, "Portfolio temporarily unavailable", http.StatusServiceUnavailable)
}
func (h *apiHandler) publicImage(w http.ResponseWriter, r *http.Request, slug string) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/images/"), "/")
	if len(parts) != 3 || h.config.Portfolio.Media == nil {
		http.NotFound(w, r)
		return
	}
	index, err := strconv.Atoi(parts[2])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body, size, kind, err := h.config.Portfolio.PublicImage(r.Context(), slug, parts[0], parts[1], index)
	if err != nil {
		h.publicError(w, r, err)
		return
	}
	defer body.Close()
	// Revalidate every reuse so unpublishing revokes access, including cached images.
	w.Header().Set("Cache-Control", "private, no-cache")
	tag := `"` + parts[0] + "/" + parts[1] + "/" + strconv.Itoa(index) + `"`
	w.Header().Set("ETag", tag)
	if matchesETag(r.Header.Get("If-None-Match"), tag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", kind)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if r.Method == http.MethodHead {
		return
	}
	if _, err = io.Copy(w, body); err != nil {
		h.logger.Error(err, "stream portfolio image")
	}
}

func matchesETag(value, tag string) bool {
	for _, candidate := range strings.Split(value, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == tag {
			return true
		}
	}
	return false
}
