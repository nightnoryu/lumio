package app

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"

	"lumio/internal/domain"
)

type MediaStore interface {
	ReservePhoto(context.Context, string, domain.Photo, domain.MediaLimits) error
	Photos(context.Context, string, string) ([]domain.Photo, error)
	Photo(context.Context, string, string, string) (domain.Photo, error)
	QueuePhoto(context.Context, string, string, string) error
	DeletePhoto(context.Context, string, string, string) error
	ClaimPhoto(context.Context) (domain.Photo, error)
	FinishPhoto(context.Context, domain.Photo, int, int, string) error
	FailPhoto(context.Context, domain.Photo) error
	CleanupPhoto(context.Context) (domain.Photo, error)
	PurgePhoto(context.Context, domain.Photo) error
}
type ObjectStore interface {
	Open(context.Context, string) (io.ReadCloser, int64, error)
	UploadURL(context.Context, string, string, int64) (string, error)
	Check(context.Context, string, string, int64) error
	Promote(context.Context, string, string) error
	Download(context.Context, string, io.Writer, int64) error
	Put(context.Context, string, string, io.Reader) error
	DeletePrefix(context.Context, string) error
	PreviewURL(context.Context, string) (string, error)
	PruneVariants(context.Context, string, string) error
}
type Media struct {
	Store   MediaStore
	Objects ObjectStore
	Limits  domain.MediaLimits
}

func (m *Media) Create(ctx context.Context, user, site, name, kind, watermark string, size int64) (domain.Photo, string, error) {
	if err := domain.ValidatePhoto(name, kind, watermark, size, m.Limits); err != nil {
		return domain.Photo{}, "", err
	}
	id := uuid.NewString()
	p := domain.Photo{ID: id, SiteID: site, Name: name, Size: size, ContentType: kind, Status: "uploading", Original: "media/" + id + "/original", Watermark: watermark, ExpiresAt: time.Now().Add(15 * time.Minute)}
	if err := m.Store.ReservePhoto(ctx, user, p, m.Limits); err != nil {
		return p, "", err
	}
	url, err := m.Objects.UploadURL(ctx, "uploads/"+p.ID+"/original", kind, size)
	return p, url, err
}
func (m *Media) Complete(ctx context.Context, user, site, id string) error {
	p, err := m.Store.Photo(ctx, user, site, id)
	if err != nil {
		return err
	}
	if p.Status == "queued" || p.Status == "processing" || p.Status == "ready" {
		return nil
	}
	if p.Status != "uploading" || time.Now().After(p.ExpiresAt) {
		return domain.ErrConflict
	}
	ctx, cancel := context.WithDeadline(ctx, p.ExpiresAt)
	defer cancel()
	if err = m.Objects.Check(ctx, "uploads/"+p.ID+"/original", p.ContentType, p.Size); err != nil {
		return err
	}
	if err = m.Objects.Promote(ctx, "uploads/"+p.ID+"/original", p.Original); err != nil {
		return err
	}
	return m.Store.QueuePhoto(ctx, user, site, id)
}
