package domain

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var ErrDraftConflict = errors.New("draft version conflict")

type Draft struct {
	Language       string             `json:"language,omitempty"`
	SEOTitle       string             `json:"seoTitle"`
	SEODescription string             `json:"seoDescription"`
	Version        int64              `json:"version"`
	DisplayName    string             `json:"displayName"`
	Biography      string             `json:"biography"`
	Location       string             `json:"location"`
	Specialization string             `json:"specialization"`
	ProfilePhoto   string             `json:"profilePhoto"`
	CoverPhoto     string             `json:"coverPhoto"`
	Template       string             `json:"template"`
	Typography     string             `json:"typography"`
	Colour         string             `json:"colour"`
	Layout         string             `json:"layout"`
	HidePrices     bool               `json:"hidePrices"`
	Photos         []DraftPhoto       `json:"photos"`
	Services       []PortfolioService `json:"services"`
	Contacts       []Contact          `json:"contacts"`
}
type DraftPhoto struct {
	ID       string `json:"id"`
	Alt      string `json:"alt"`
	Category string `json:"category"`
}
type PortfolioService struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       string `json:"price"`
}
type Contact struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}
type DraftError struct{ Message string }

func (e *DraftError) Error() string     { return e.Message }
func InvalidDraft(message string) error { return &DraftError{Message: message} }
func EmptyDraft() Draft {
	return Draft{Language: "en", Template: "gallery", Typography: "serif", Colour: "light", Layout: "grid", Photos: []DraftPhoto{}, Services: []PortfolioService{}, Contacts: []Contact{}}
}
func (d Draft) Validate() error {
	if d.Language != "" && d.Language != "en" && d.Language != "ru" {
		return InvalidDraft("Choose a supported language.")
	}
	if d.Version < 0 || len(d.Photos) > 100 || len(d.Services) > 20 || len(d.Contacts) > 12 {
		return InvalidDraft("Draft exceeds the supported limits.")
	}
	for _, choice := range []struct {
		value   string
		allowed string
	}{{d.Template, "gallery editorial"}, {d.Typography, "serif sans"}, {d.Colour, "light dark warm"}, {d.Layout, "grid column"}} {
		if !slices.Contains(strings.Fields(choice.allowed), choice.value) || choice.value == "" {
			return InvalidDraft("Choose a supported appearance option.")
		}
	}
	for _, field := range []struct {
		name, value string
		limit       int
	}{{"SEO title", d.SEOTitle, 120}, {"SEO description", d.SEODescription, 300}, {"Display name", d.DisplayName, 120}, {"Biography", d.Biography, 4000}, {"Location", d.Location, 160}, {"Specialization", d.Specialization, 160}} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.limit {
			return InvalidDraft(fmt.Sprintf("%s is too long.", field.name))
		}
	}
	seen := map[string]bool{}
	for _, p := range d.Photos {
		if !canonicalPhotoID(p.ID) || seen[p.ID] {
			return InvalidDraft("Select each photograph only once.")
		}
		seen[p.ID] = true
		if utf8.RuneCountInString(p.Alt) > 300 || utf8.RuneCountInString(p.Category) > 80 {
			return InvalidDraft("Photo descriptions or categories are too long.")
		}
	}
	for _, id := range []string{d.ProfilePhoto, d.CoverPhoto} {
		if id != "" {
			if !canonicalPhotoID(id) {
				return InvalidDraft("Choose a valid profile or cover photograph.")
			}
		}
	}
	for _, s := range d.Services {
		if strings.TrimSpace(s.Name) == "" || utf8.RuneCountInString(s.Name) > 120 || utf8.RuneCountInString(s.Description) > 1000 || utf8.RuneCountInString(s.Price) > 100 {
			return InvalidDraft("Each service needs a name; check service text lengths.")
		}
	}
	for _, c := range d.Contacts {
		if err := c.Validate(); err != nil {
			return err
		}
	}

	return nil
}
func (c Contact) Validate() error {
	u, err := url.Parse(c.URL)
	if strings.TrimSpace(c.Label) == "" || utf8.RuneCountInString(c.Label) > 60 || len(c.URL) > 500 || err != nil || strings.ContainsAny(c.URL, "\r\n") {
		return InvalidDraft("Enter a contact label and valid link.")
	}
	switch u.Scheme {
	case "https":
		if u.Hostname() == "" || u.User != nil {
			return InvalidDraft("Contact websites need a valid HTTPS address.")
		}
	case "mailto":
		if _, err := NormalizeEmail(u.Opaque); err != nil || u.RawQuery != "" {
			return InvalidDraft("Enter a valid email link, such as mailto:hello@example.com.")
		}
	case "tel":
		if u.Opaque == "" || strings.Trim(u.Opaque, "+0123456789 ()-") != "" {
			return InvalidDraft("Enter a valid telephone link, such as tel:+123456789.")
		}
	default:
		return InvalidDraft("Use HTTPS, mailto: or tel: contact links.")
	}
	return nil
}
func (d Draft) PhotoIDs() []string {
	ids := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, p := range d.Photos {
		add(p.ID)
	}
	add(d.ProfilePhoto)
	add(d.CoverPhoto)
	return ids
}

func canonicalPhotoID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}
