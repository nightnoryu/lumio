package portfolio

import (
	"strings"
	"unicode/utf8"

	"lumio/internal/domain"
)

type Metadata struct{ Canonical, Title, Description, Image, ImageAlt string }

func pageMetadata(d domain.Draft, images map[string]string, options []Metadata) Metadata {
	meta := Metadata{}
	if len(options) > 0 {
		meta = options[0]
	}
	meta.Title = strings.TrimSpace(d.SEOTitle)
	if meta.Title == "" {
		meta.Title = d.DisplayName + " — Photography"
	}
	meta.Description = strings.TrimSpace(d.SEODescription)
	if meta.Description == "" {
		meta.Description = strings.TrimSpace(d.Biography)
		if utf8.RuneCountInString(meta.Description) > 300 {
			meta.Description = string([]rune(meta.Description)[:297]) + "…"
		}
	}
	id := d.CoverPhoto
	if id == "" && len(d.Photos) > 0 {
		id = d.Photos[0].ID
	}
	meta.Image = images[id]
	if meta.Image != "" && strings.HasPrefix(meta.Image, "/") {
		meta.Image = strings.TrimSuffix(meta.Canonical, "/") + meta.Image
	}
	meta.ImageAlt = "Photography by " + d.DisplayName
	for _, p := range d.Photos {
		if p.ID == id {
			meta.ImageAlt = p.Alt
			break
		}
	}
	return meta
}
