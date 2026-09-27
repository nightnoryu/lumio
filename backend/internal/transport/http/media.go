package httptransport

import (
	"context"
	"encoding/json"

	"lumio/api/server/publicapi"
	"lumio/internal/domain"
)

func (h *apiHandler) ListPhotos(ctx context.Context, p publicapi.ListPhotosParams) (publicapi.Photos, error) {
	photos, err := h.config.Media.Store.Photos(ctx, request(ctx).user.ID, p.ID.String())
	result := make(publicapi.Photos, 0, len(photos))
	for _, photo := range photos {
		result = append(result, publicapi.Photo{ID: photo.ID, Name: photo.Name, Status: photo.Status, Width: photo.Width, Height: photo.Height, Size: photo.Size})
	}
	return result, err
}
func (h *apiHandler) CreateUpload(ctx context.Context, r *publicapi.UploadInput, p publicapi.CreateUploadParams) (*publicapi.Upload, error) {
	photo, url, err := h.config.Media.Create(ctx, request(ctx).user.ID, p.ID.String(), r.Name, string(r.ContentType), r.Watermark, r.Size)
	return &publicapi.Upload{ID: photo.ID, URL: url}, err
}
func (h *apiHandler) CompleteUpload(ctx context.Context, p publicapi.CompleteUploadParams) error {
	return h.config.Media.Complete(ctx, request(ctx).user.ID, p.ID.String(), p.PhotoId.String())
}
func (h *apiHandler) DeletePhoto(ctx context.Context, p publicapi.DeletePhotoParams) error {
	return h.config.Media.Store.DeletePhoto(ctx, request(ctx).user.ID, p.ID.String(), p.PhotoId.String())
}
func (h *apiHandler) PreviewPhoto(ctx context.Context, p publicapi.PreviewPhotoParams) (*publicapi.Preview, error) {
	photo, err := h.config.Media.Store.Photo(ctx, request(ctx).user.ID, p.ID.String(), p.PhotoId.String())
	if err != nil {
		return nil, err
	}
	if photo.Status != "ready" {
		return nil, domain.ErrConflict
	}
	var keys []string
	if err = json.Unmarshal([]byte(photo.Variants), &keys); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, domain.ErrNotFound
	}
	url, err := h.config.Media.Objects.PreviewURL(ctx, keys[0])
	return &publicapi.Preview{URL: url}, err
}
