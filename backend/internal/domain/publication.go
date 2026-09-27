package domain

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"
)

type Revision struct {
	ID     string
	Draft  Draft
	Images map[string]PublishedImage
}
type PublishedImage struct {
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Variants []Variant `json:"variants"`
}
type Variant struct {
	Key    string `json:"key"`
	Width  int    `json:"width"`
	Format string `json:"format"`
}

func (d Draft) ValidatePublication() error {
	if err := d.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(d.DisplayName) == "" || strings.TrimSpace(d.Biography) == "" || len(d.Photos) == 0 || len(d.Contacts) == 0 {
		return InvalidDraft("Before publishing, add your display name, biography, at least one photograph and a contact link.")
	}
	for _, photo := range d.Photos {
		if strings.TrimSpace(photo.Alt) == "" {
			return InvalidDraft("Add an image description to every portfolio photograph before publishing.")
		}
	}
	return nil
}
func SnapshotImage(p Photo) (PublishedImage, error) {
	result := PublishedImage{Width: p.Width, Height: p.Height}
	if p.Status != "ready" || p.Width < 1 || p.Height < 1 {
		return result, InvalidDraft("Selected photographs must finish processing before publication.")
	}
	var keys []string
	if err := json.Unmarshal([]byte(p.Variants), &keys); err != nil {
		return result, err
	}
	if len(keys) == 0 {
		return result, InvalidDraft("A selected photograph has no processed images. Upload it again.")
	}
	prefix := "media/" + p.ID + "/v1/" + p.Lease + "/"
	hasJPEG := false
	for _, key := range keys {
		name := strings.TrimPrefix(key, prefix)
		ext := path.Ext(name)
		width, err := strconv.Atoi(strings.TrimSuffix(name, ext))
		if p.Lease == "" || !strings.HasPrefix(key, prefix) || err != nil || width < 1 || width > p.Width || width > 2400 || (ext != ".jpg" && ext != ".webp") {
			return result, InvalidDraft("A selected photograph has invalid processed images. Upload it again.")
		}
		result.Variants = append(result.Variants, Variant{Key: key, Width: width, Format: strings.TrimPrefix(ext, ".")})
		hasJPEG = hasJPEG || ext == ".jpg"
	}
	if !hasJPEG {
		return result, InvalidDraft("A selected photograph is missing its JPEG fallback.")
	}
	return result, nil
}
