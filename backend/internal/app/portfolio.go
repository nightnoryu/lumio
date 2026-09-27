package app

import (
	"context"
	"encoding/json"

	"lumio/internal/domain"
)

type PortfolioStore interface {
	Draft(context.Context, string, string) (domain.Draft, error)
	SaveDraft(context.Context, string, string, domain.Draft) (domain.Draft, error)
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
