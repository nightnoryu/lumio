package domain

import (
	"encoding/json"
	"testing"
)

func TestPublicationRequiresCompleteDraft(t *testing.T) {
	d := EmptyDraft()
	if d.ValidatePublication() == nil {
		t.Fatal("empty portfolio published")
	}
	d.DisplayName = "Anna"
	d.Biography = "Portrait photographer"
	d.Contacts = []Contact{{Label: "Email", URL: "mailto:anna@example.com"}}
	d.Photos = []DraftPhoto{{ID: "5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f"}}
	if d.ValidatePublication() == nil {
		t.Fatal("missing alt text accepted")
	}
	d.Photos[0].Alt = "Portrait in sunlight"
	if err := d.ValidatePublication(); err != nil {
		t.Fatal(err)
	}
}
func TestSnapshotExcludesOriginalsAndOtherPhotos(t *testing.T) {
	p := Photo{ID: "5ae4ce6a-7f61-4c3c-8a74-b3e1dfed634f", Lease: "attempt", Status: "ready", Width: 1000, Height: 700}
	prefix := "media/" + p.ID + "/v1/attempt/"
	for _, key := range []string{"media/" + p.ID + "/original", "uploads/" + p.ID + "/original", "media/other/v1/attempt/960.jpg", prefix + "../../original", prefix + "2000.jpg", prefix + "960.svg"} {
		raw, _ := json.Marshal([]string{key})
		p.Variants = string(raw)
		if _, err := SnapshotImage(p); err == nil {
			t.Errorf("accepted %s", key)
		}
	}
	raw, _ := json.Marshal([]string{prefix + "960.webp", prefix + "960.jpg"})
	p.Variants = string(raw)
	if _, err := SnapshotImage(p); err != nil {
		t.Fatal(err)
	}
	p.Status = "processing"
	if _, err := SnapshotImage(p); err == nil {
		t.Fatal("processing photo accepted")
	}
}
