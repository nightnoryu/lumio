package domain

import (
	"errors"
	"time"
)

var ErrMediaQuota = errors.New("media quota reached")
var ErrMediaInput = errors.New("invalid media input")

type Photo struct {
	ID          string    `db:"id" json:"id"`
	SiteID      string    `db:"site_id" json:"-"`
	Name        string    `db:"name" json:"name"`
	Size        int64     `db:"size" json:"size"`
	ContentType string    `db:"content_type" json:"contentType"`
	Status      string    `db:"status" json:"status"`
	Original    string    `db:"original" json:"-"`
	Watermark   string    `db:"watermark" json:"watermark"`
	Width       int       `db:"width" json:"width"`
	Height      int       `db:"height" json:"height"`
	Variants    string    `db:"variants" json:"-"`
	ExpiresAt   time.Time `db:"expires_at" json:"expiresAt"`
	Attempts    int       `db:"attempts" json:"-"`
	AvailableAt time.Time `db:"available_at" json:"-"`
	Lease       string    `db:"lease" json:"-"`
}
type MediaLimits struct {
	FileBytes, StorageBytes int64
	Photos                  int
}

func ValidatePhoto(name, contentType, watermark string, size int64, limits MediaLimits) error {
	if name == "" || len(name) > 255 || size <= 0 || size > limits.FileBytes || len(watermark) > 80 {
		return ErrMediaInput
	}
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return ErrMediaInput
	}
	for _, r := range watermark {
		if r < 32 || r > 126 {
			return ErrMediaInput
		}
	}
	return nil
}
