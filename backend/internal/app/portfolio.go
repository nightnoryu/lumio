package app

import (
	"context"
	"encoding/json"
	"io"

	"lumio/internal/domain"
)

type PortfolioStore interface {
	Draft(context.Context, string, string) (domain.Draft, error)
	SaveDraft(context.Context, string, string, domain.Draft) (domain.Draft, error)
	Publish(context.Context, string, string, int64) (int64, error)
	Unpublish(context.Context, string, string) error
	PublicationVersion(context.Context, string, string) (int64, error)
	Published(context.Context, string) (domain.Revision, error)
}
type Portfolio struct {
	Store PortfolioStore
	Media *Media
}

func (p *Portfolio) Draft(ctx context.Context, user, site string) (domain.Draft, error) {
	return p.Store.Draft(ctx, user, site)
}
func (p *Portfolio) Save(ctx context.Context, user, site string, d domain.Draft) (domain.Draft, error) {
	if err := d.Validate(); err != nil {
		return d, err
	}
	return p.Store.SaveDraft(ctx, user, site, d)
}
func (p *Portfolio) Preview(ctx context.Context, user, site string) (domain.Draft, map[string]string, error) {
	d, err := p.Draft(ctx, user, site)
	if err != nil {
		return d, nil, err
	}
	urls := map[string]string{}
	for _, id := range d.PhotoIDs() {
		photo, err := p.Media.Store.Photo(ctx, user, site, id)
		if err != nil {
			return d, nil, err
		}
		if photo.Status != "ready" {
			return d, nil, domain.InvalidDraft("A selected photograph is no longer ready. Update your photo selection.")
		}
		var keys []string
		if err = json.Unmarshal([]byte(photo.Variants), &keys); err != nil {
			return d, nil, err
		}
		if len(keys) == 0 {
			return d, nil, domain.ErrNotFound
		}
		urls[id], err = p.Media.Objects.PreviewURL(ctx, keys[len(keys)-1])
		if err != nil {
			return d, nil, err
		}
	}
	return d, urls, nil
}

func (p *Portfolio) Publish(ctx context.Context, user, site string, version int64) (int64, error) {
	return p.Store.Publish(ctx, user, site, version)
}
func (p *Portfolio) Unpublish(ctx context.Context, user, site string) error {
	return p.Store.Unpublish(ctx, user, site)
}
func (p *Portfolio) PublicationVersion(ctx context.Context, user, site string) (int64, error) {
	return p.Store.PublicationVersion(ctx, user, site)
}
func (p *Portfolio) Published(ctx context.Context, slug string) (domain.Revision, error) {
	return p.Store.Published(ctx, slug)
}
func (p *Portfolio) PublicImage(ctx context.Context, slug, revision, photo string, index int) (body io.ReadCloser, size int64, kind string, err error) {
	r, err := p.Published(ctx, slug)
	if err != nil {
		return nil, 0, "", err
	}
	image, ok := r.Images[photo]
	if r.ID != revision || !ok || index < 0 || index >= len(image.Variants) {
		return nil, 0, "", domain.ErrNotFound
	}
	variant := image.Variants[index]
	body, size, err = p.Media.Objects.Open(ctx, variant.Key)
	kind = "image/jpeg"
	if variant.Format == "webp" {
		kind = "image/webp"
	}
	return body, size, kind, err
}
