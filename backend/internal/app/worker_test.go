package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"lumio/internal/domain"
)

type expiredClaim struct{ MediaStore }

func (expiredClaim) ClaimPhoto(context.Context) (domain.Photo, error) {
	return domain.Photo{ID: "stale", Attempts: 1, Lease: "old-attempt", AvailableAt: time.Now().Add(-time.Minute)}, nil
}

type forbiddenProcessor struct{ t *testing.T }

func (p forbiddenProcessor) Process(context.Context, domain.Photo, ObjectStore) (width, height int, manifest string, err error) {
	p.t.Fatal("stale worker resumed processing")
	return 0, 0, "", nil
}
func TestExpiredClaimCannotTouchObjects(t *testing.T) {
	observed := false
	media := &Media{Store: expiredClaim{}, Observe: func(operation, outcome string, duration time.Duration) {
		observed = true
		if operation != "process" || outcome != "error" || duration < 0 {
			t.Fatalf("incorrect operation: %s %s", operation, outcome)
		}
	}}
	// Nil object store deliberately catches any S3 access by the suspended attempt.
	if err := media.processNext(t.Context(), forbiddenProcessor{t}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected expired attempt, got %v", err)
	}
	if !observed {
		t.Fatal("failed job was not observed")
	}
}
