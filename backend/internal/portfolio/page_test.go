package portfolio

import (
	"bytes"
	"strings"
	"testing"

	"lumio/internal/domain"
)

func TestTemplatesRenderSavedContent(t *testing.T) {
	for _, name := range []string{"gallery", "editorial", studioTemplate} {
		d := domain.EmptyDraft()
		d.Template = name
		d.DisplayName = "<script>alert(1)</script>"
		d.CoverPhoto = "cover"
		d.Photos = []domain.DraftPhoto{{ID: "photo", Alt: "Portrait", Category: "People"}}
		d.Services = []domain.PortfolioService{{Name: "Portrait session", Price: "€250"}}
		d.HidePrices = true
		var out bytes.Buffer
		if err := Page(d, map[string]string{"cover": "https://images.example/cover.jpg", "photo": "https://images.example/photo.jpg"}).Render(t.Context(), &out); err != nil {
			t.Fatal(err)
		}
		html := out.String()
		for _, content := range []string{"&lt;script&gt;", "Portrait session", "People", "https://images.example/photo.jpg", `name="viewport"`, "--paper: #faf9f6", `href="#photographs"`} {
			if !strings.Contains(html, content) {
				t.Errorf("%s missing %s", name, content)
			}
		}
		if strings.Contains(html, "<script>") || strings.Contains(html, "€250") {
			t.Fatal("unsafe HTML or hidden price rendered")
		}
		intro, cover := strings.Index(html, `<header class="intro">`), strings.Index(html, `<img class="cover"`)
		if intro < 0 || cover < intro {
			t.Fatal("incorrect template section order")
		}
	}
}

func TestPortfolioLanguage(t *testing.T) {
	for _, language := range []string{"", "en", "ru"} {
		for _, template := range []string{"gallery", "editorial", studioTemplate} {
			t.Run(language+"/"+template, func(t *testing.T) {
				d := domain.EmptyDraft()
				d.Language = language
				d.Template = template
				d.DisplayName = "Anna"
				d.Biography = "Original biography"
				d.Photos = []domain.DraftPhoto{{ID: "photo", Alt: "Original description"}}
				d.Services = []domain.PortfolioService{{Name: "Original service"}}
				var out bytes.Buffer
				if err := Page(d, map[string]string{"photo": "/photo.jpg"}).Render(t.Context(), &out); err != nil {
					t.Fatal(err)
				}
				expected := []string{`lang="en"`, "Services", "Previous photograph", "Anna — Photography"}
				if language == "ru" {
					expected = []string{`lang="ru"`, "Услуги", "Предыдущая фотография", "Anna — Фотография", "Посмотреть фотографию: Original description"}
				}
				for _, text := range append(expected, "Original biography", "Original service", "Original description") {
					if !strings.Contains(out.String(), text) {
						t.Errorf("missing %q", text)
					}
				}
			})
		}
	}
}
