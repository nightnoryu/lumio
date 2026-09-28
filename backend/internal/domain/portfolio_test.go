package domain

import "testing"

func TestDraftValidation(t *testing.T) {
	for _, link := range []string{"https://t.me/anna", "https://wa.me/123456789", "mailto:anna@example.com", "tel:+123456789"} {
		d := EmptyDraft()
		d.Contacts = []Contact{{Label: "Contact", URL: link}}
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", link, err)
		}
	}
	for _, link := range []string{"javascript:alert(1)", "data:text/html,hi", "https://", "https://user:pass@example.com", "mailto:not-an-email", "tel:abc", "https://example.com\r\nInjected:yes"} {
		d := EmptyDraft()
		d.Contacts = []Contact{{Label: "Contact", URL: link}}
		if d.Validate() == nil {
			t.Errorf("accepted %q", link)
		}
	}
	d := EmptyDraft()
	d.Photos = []DraftPhoto{{ID: "5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f"}, {ID: "5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f"}}
	if d.Validate() == nil {
		t.Fatal("accepted duplicate photograph")
	}
	d = EmptyDraft()
	d.Template = "gallery editorial"
	if d.Validate() == nil {
		t.Fatal("accepted combined template")
	}
}

func TestDraftRejectsNoncanonicalPhotoReferences(t *testing.T) {
	for _, id := range []string{"5AE4CE6A-7F61-4C3C-8A74-B3E1DFED634F", "5ae4ce6a7f614c3c8a74b3e1dfed634f", "{5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f}"} {
		for _, field := range []string{"photos", "profile", "cover"} {
			d := EmptyDraft()
			switch field {
			case "photos":
				d.Photos = []DraftPhoto{{ID: id}}
			case "profile":
				d.ProfilePhoto = id
			case "cover":
				d.CoverPhoto = id
			}
			if d.Validate() == nil {
				t.Errorf("accepted noncanonical %s reference %q", field, id)
			}
		}
	}
}

func TestDraftLanguageValidation(t *testing.T) {
	for _, language := range []string{"", "en", "ru", "fr", "RU", "en ru"} {
		d := EmptyDraft()
		d.Language = language
		valid := language == "" || language == "en" || language == "ru"
		if (d.Validate() == nil) != valid {
			t.Errorf("unexpected validation for language %q", language)
		}
	}
}

func TestDraftTemplates(t *testing.T) {
	for _, template := range []string{"gallery", "editorial", "studio"} {
		d := EmptyDraft()
		d.Template = template
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", template, err)
		}
	}
}
