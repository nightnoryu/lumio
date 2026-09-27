package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lumio/internal/domain"
)

type Processor interface {
	Process(context.Context, domain.Photo, ObjectStore) (int, int, string, error)
}

func (m *Media) Work(ctx context.Context, processor Processor, report func(error)) error {
	for ctx.Err() == nil {
		for _, err := range []error{m.processNext(ctx, processor), m.cleanupNext(ctx)} {
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				report(err)
			}
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return ctx.Err()
}
func (m *Media) processNext(ctx context.Context, processor Processor) error {
	p, err := m.Store.ClaimPhoto(ctx)
	if err != nil {
		return err
	}
	if p.Attempts > 3 {
		return m.Store.FailPhoto(ctx, p)
	}
	// Anchor the deadline to the database claim, including time spent suspended.
	job, cancel := context.WithDeadline(ctx, p.AvailableAt.Add(-7*time.Minute))
	defer cancel()
	if err = job.Err(); err != nil {
		return err
	}
	w, h, variants, err := processor.Process(job, p, m.Objects)
	if err != nil {
		cleanup, stop := context.WithTimeout(ctx, time.Minute)
		defer stop()
		return errors.Join(fmt.Errorf("photo %s: %w", p.ID, err), m.Store.FailPhoto(ctx, p), m.Objects.DeletePrefix(cleanup, "media/"+p.ID+"/v1/"+p.Lease+"/"))
	}
	if err = m.Store.FinishPhoto(job, p, w, h, variants); err != nil {
		return err
	}
	return m.Objects.PruneVariants(job, p.ID, p.Lease)
}
func (m *Media) cleanupNext(ctx context.Context) error {
	p, err := m.Store.CleanupPhoto(ctx)
	if err != nil {
		return err
	}
	job, cancel := context.WithDeadline(ctx, p.AvailableAt.Add(-9*time.Minute))
	defer cancel()
	if err = m.Objects.DeletePrefix(job, "media/"+p.ID+"/"); err != nil {
		return err
	}
	if err = m.Objects.DeletePrefix(job, "uploads/"+p.ID+"/"); err != nil {
		return err
	}
	return m.Store.PurgePhoto(job, p)
}
