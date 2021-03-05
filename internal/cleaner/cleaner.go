package cleaner

import (
	"context"
	"log"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
)

// BulkDeleter whatever requires deleting
type BulkDeleter interface {
	Delete(ctx context.Context, id ...string) error
}

// SessionRepository interfaces with session storage
type SessionRepository interface {
	BulkDeleter
	Get(ctx context.Context, opts ...repo.GetOpt) ([]repo.Session, error)
}

// Cleaner finds old records using session repository and then deletes items older than StorageMaxAge
type Cleaner struct {
	StorageMaxAge time.Duration
	Sessions      SessionRepository
	Events        BulkDeleter
}

// New return Cleaner instance
func New(t time.Duration, session SessionRepository, events BulkDeleter) *Cleaner {
	return &Cleaner{t, session, events}
}

// Run run ticker which cleans-up old registers
func (c *Cleaner) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	if err := c.run(ctx); err != nil {
		return err
	}

	select {
	case <-ticker.C:
		if err := c.run(ctx); err != nil {
			return err
		}
	case <-ctx.Done():
		return nil
	}

	return nil
}

func (c *Cleaner) run(ctx context.Context) error {
	t := time.Now().Add(-c.StorageMaxAge)

	sessions, err := c.Sessions.Get(ctx, repo.WithUpdatedAtUntil(t))
	if err != nil {
		return err
	}

	ids := make([]string, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}

	if err := c.Events.Delete(ctx, ids...); err != nil {
		log.Println(err)
		return err
	}

	return c.Sessions.Delete(ctx, ids...)
}
