//go:build e2e

package e2e

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"lumio/internal/domain"
	"lumio/internal/infrastructure/postgres"
)

func TestMediaReservationsAndLeases(t *testing.T) {
	f := newIdentityFixture(t)
	f.register("media@example.com")
	f.register("other@example.com")
	user, err := f.service.Store.UserByEmail(t.Context(), "media@example.com")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.service.Store.UserByEmail(t.Context(), "other@example.com")
	if err != nil {
		t.Fatal(err)
	}
	site, err := f.service.CreateSite(t.Context(), user.ID, "media")
	if err != nil {
		t.Fatal(err)
	}
	store := &postgres.Store{DB: f.db.TransactionalClient()}
	limits := domain.MediaLimits{Photos: 1, FileBytes: 100, StorageBytes: 100}
	makePhoto := func() domain.Photo {
		id := uuid.NewString()
		return domain.Photo{ID: id, SiteID: site.ID, Name: "photo.jpg", Size: 60, ContentType: "image/jpeg", Original: "media/" + id + "/original", ExpiresAt: time.Now().Add(time.Minute)}
	}
	if err = store.ReservePhoto(t.Context(), other.ID, makePhoto(), limits); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("ownership bypass", err)
	}
	reserveConcurrent(t, store, user.ID, makePhoto, limits)
	photos, err := store.Photos(t.Context(), user.ID, site.ID)
	if err != nil || len(photos) != 1 {
		t.Fatal(photos, err)
	}
	p := photos[0]
	if _, err = store.Photo(t.Context(), other.ID, site.ID, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("photo leaked", err)
	}
	if err = store.QueuePhoto(t.Context(), user.ID, site.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	first, err := store.ClaimPhoto(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimPhoto(t.Context()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("double claim", err)
	}
	if _, err = f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE photo SET available_at=now()-interval '1 second' WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	second, err := store.ClaimPhoto(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first.Lease == second.Lease || second.Attempts != 2 {
		t.Fatal("lease not replaced")
	}
	if err = store.FinishPhoto(t.Context(), first, 10, 10, "[]"); !errors.Is(err, domain.ErrConflict) {
		t.Fatal("stale worker committed", err)
	}
	if err = store.FinishPhoto(t.Context(), second, 20, 10, "[]"); err != nil {
		t.Fatal(err)
	}
	current, err := store.Photo(t.Context(), user.ID, site.ID, p.ID)
	if err != nil || current.Status != "ready" || current.Width != 20 {
		t.Fatal(current, err)
	}
	if err = store.DeletePhoto(t.Context(), other.ID, site.ID, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("unauthorized deletion", err)
	}
	if err = store.DeletePhoto(t.Context(), user.ID, site.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.ReservePhoto(t.Context(), user.ID, makePhoto(), limits); !errors.Is(err, domain.ErrMediaQuota) {
		t.Fatal("quota released before cleanup", err)
	}
	if _, err = store.CleanupPhoto(t.Context()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cleanup before URL expiry", err)
	}
	if _, err = f.db.TransactionalClient().ExecContext(t.Context(), `UPDATE photo SET available_at=now()-interval '1 second' WHERE id=$1`, p.ID); err != nil {
		t.Fatal(err)
	}
	deleted, err := store.CleanupPhoto(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err = store.PurgePhoto(t.Context(), deleted); err != nil {
		t.Fatal(err)
	}
	if err = store.ReservePhoto(t.Context(), user.ID, makePhoto(), limits); err != nil {
		t.Fatal("quota not released", err)
	}
}

func reserveConcurrent(t *testing.T, store *postgres.Store, userID string, makePhoto func() domain.Photo, limits domain.MediaLimits) {
	t.Helper()
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() { results <- store.ReservePhoto(t.Context(), userID, makePhoto(), limits) })
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, domain.ErrMediaQuota) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("quota race: %d reservations", successes)
	}
}
