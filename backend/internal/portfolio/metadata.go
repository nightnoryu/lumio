package portfolio

import (
	"strings"
	"unicode/utf8"

	"lumio/internal/domain"
)

const (
	editorialTemplate = "editorial"
	studioTemplate    = "studio"
)

type Metadata struct {
	Canonical, Title, Description, Image, ImageAlt string
	Sources                                        map[string]string
}

func pageMetadata(d domain.Draft, images map[string]string, options []Metadata) Metadata {
	meta := Metadata{}
	if len(options) > 0 {
		meta = options[0]
	}
	meta.Title = strings.TrimSpace(d.SEOTitle)
	if meta.Title == "" {
		meta.Title = d.DisplayName + " — " + localize(d, "Photography")
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
	meta.ImageAlt = localize(d, "Photography by ") + d.DisplayName
	for _, p := range d.Photos {
		if p.ID == id {
			meta.ImageAlt = p.Alt
			break
		}
	}
	return meta
}

func gallerySizes(d domain.Draft, index int) string {
	if d.Layout == "column" {
		return "(max-width: 640px) 88vw, (max-width: 1000px) 90vw, 900px"
	}
	if d.Template == studioTemplate {
		return "(max-width: 800px) 44vw, (max-width: 1600px) 30vw, 460px"
	}
	if d.Template == editorialTemplate && index%3 == 0 {
		return "100vw"
	}
	return "(max-width: 640px) 88vw, (max-width: 1600px) 45vw, 704px"
}
func imageLoading(d domain.Draft, index int) string {
	if d.CoverPhoto == "" && index == 0 {
		return "eager"
	}
	return "lazy"
}
