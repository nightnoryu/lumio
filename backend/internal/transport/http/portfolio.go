package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"lumio/api/server/publicapi"
	"lumio/internal/domain"
	"lumio/internal/portfolio"
)

func apiDraft(d domain.Draft) (*publicapi.Draft, error) {
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var result publicapi.Draft
	err = json.Unmarshal(raw, &result)
	return &result, err
}
func (h *apiHandler) GetDraft(ctx context.Context, p publicapi.GetDraftParams) (*publicapi.Draft, error) {
	if h.config.Portfolio == nil {
		return nil, domain.ErrNotFound
	}
	d, err := h.config.Portfolio.Draft(ctx, request(ctx).user.ID, p.ID.String())
	if err != nil {
		return nil, err
	}
	return apiDraft(d)
}
func (h *apiHandler) SaveDraft(ctx context.Context, r *publicapi.Draft, p publicapi.SaveDraftParams) (*publicapi.Draft, error) {
	if h.config.Portfolio == nil {
		return nil, domain.ErrNotFound
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var d domain.Draft
	if err = json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	d, err = h.config.Portfolio.Save(ctx, request(ctx).user.ID, p.ID.String(), d)
	if err != nil {
		return nil, err
	}
	return apiDraft(d)
}
func (h *apiHandler) previewPortfolio(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self' "+h.config.StorageOrigin+"; style-src 'unsafe-inline'; script-src 'self'; frame-ancestors 'self'; base-uri 'none'; form-action 'none'")
	origin, err := url.Parse(h.config.Origin)
	if err != nil || r.Host != origin.Host {
		writeError(w, 403, "Invalid dashboard host")
		return
	}
	if h.config.Service == nil || h.config.Portfolio == nil || h.config.Portfolio.Media == nil {
		writeError(w, 503, "Portfolio service unavailable")
		return
	}
	state, ok := h.authorize(w, r, false)
	if !ok {
		return
	}
	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		http.NotFound(w, r)
		return
	}
	d, urls, err := h.config.Portfolio.Preview(r.Context(), state.user.ID, id.String())
	if err != nil {
		response := h.NewError(r.Context(), err)
		writeError(w, response.StatusCode, response.Response.Message)
		return
	}
	var body bytes.Buffer
	if err = portfolio.Page(d, urls).Render(r.Context(), &body); err != nil {
		response := h.NewError(r.Context(), err)
		writeError(w, response.StatusCode, response.Response.Message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(body.Bytes())
}
