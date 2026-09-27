package httptransport

import (
	"context"
	"net/url"
	"strings"

	"lumio/api/server/publicapi"
	"lumio/internal/domain"
)

func (h *apiHandler) publicationURL(slug string) string {
	origin, _ := url.Parse(h.config.Origin)
	host := slug + "." + h.config.BaseDomain
	if origin.Port() != "" {
		host += ":" + origin.Port()
	}
	return origin.Scheme + "://" + host + "/"
}
func (h *apiHandler) publicationState(ctx context.Context, site string, version int64) (*publicapi.Publication, error) {
	owned, err := h.config.Service.Site(ctx, request(ctx).user.ID, site)
	if err != nil {
		return nil, err
	}
	return &publicapi.Publication{Published: version > 0, Version: version, URL: h.publicationURL(owned.Slug)}, nil
}
func (h *apiHandler) GetPublication(ctx context.Context, p publicapi.GetPublicationParams) (*publicapi.Publication, error) {
	if h.config.Portfolio == nil {
		return nil, domain.ErrNotFound
	}
	version, err := h.config.Portfolio.PublicationVersion(ctx, request(ctx).user.ID, p.ID.String())
	if err != nil {
		return nil, err
	}
	return h.publicationState(ctx, p.ID.String(), version)
}
func (h *apiHandler) PublishSite(ctx context.Context, r *publicapi.PublishSiteReq, p publicapi.PublishSiteParams) (*publicapi.Publication, error) {
	if h.config.Portfolio == nil {
		return nil, domain.ErrNotFound
	}
	version, err := h.config.Portfolio.Publish(ctx, request(ctx).user.ID, p.ID.String(), r.Version)
	if err != nil {
		return nil, err
	}
	return h.publicationState(ctx, p.ID.String(), version)
}
func (h *apiHandler) UnpublishSite(ctx context.Context, p publicapi.UnpublishSiteParams) error {
	if h.config.Portfolio == nil {
		return domain.ErrNotFound
	}
	return h.config.Portfolio.Unpublish(ctx, request(ctx).user.ID, p.ID.String())
}
func publicSlug(host, base, port string) (string, bool) {
	if port != "" {
		suffix := ":" + port
		if !strings.HasSuffix(host, suffix) {
			return "", false
		}
		host = strings.TrimSuffix(host, suffix)
	}
	suffix := "." + base
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	slug := strings.TrimSuffix(host, suffix)
	normalized, err := domain.NormalizeSlug(slug)
	return slug, err == nil && normalized == slug
}
