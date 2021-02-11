package storage

import (
	"context"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
)

type Record struct {
	ID        string        `json:"id"`
	Events    []interface{} `json:"events"`
	UpdatedAt time.Time     `json:"updatedAt"`
	CreatedAt time.Time     `json:"createdAt"`
}

// Store defines an interface for persisting and retrieving transaction committed events.
type Store interface {
	Save(ctx context.Context, in Record) error
	Find(ctx context.Context, ID string) (*Record, error)
	FindByClientID(ctx context.Context, clientID string) ([]*Record, error)
	Close() error
}

type badgerStore struct {
	db     *badger.DB
	gcCtx  context.Context
	stopGC func()
}

// NewBadgerStore returns a store instance backed by badger db.
func NewBadgerStore(path string) (Store, error) {
	db, err := badger.Open(
		badger.DefaultOptions(path).
			WithTableLoadingMode(options.FileIO).
			WithValueLogLoadingMode(options.FileIO).
			WithNumVersionsToKeep(1).
			WithNumLevelZeroTables(1).
			WithNumLevelZeroTablesStall(2),
	)
	if err != nil {
		return nil, err
	}

	gcCtx, stopGC := context.WithCancel(context.Background())
	store := &badgerStore{
		db:     db,
		gcCtx:  gcCtx,
		stopGC: stopGC,
	}
	store.startGC()

	return store, nil
}

func (store *badgerStore) startGC() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-store.gcCtx.Done():
				return
			case <-ticker.C:
			again:
				err := store.db.RunValueLogGC(0.5)
				if err == nil && store.gcCtx.Err() == nil {
					goto again
				}
			}
		}
	}()
}

// Save persists the supplied transactions
func (store *badgerStore) Save(ctx context.Context, in Record) error {
	return nil
}

// Query fetches persisted transactions. The supplied callback is invoked with every transaction.
func (store *badgerStore) Find(ctx context.Context, ID string) (*Record, error) {
	return nil, nil
}

func (store *badgerStore) FindByClientID(ctx context.Context, clientID string) ([]*Record, error) {
	return nil, nil
}

func (store *badgerStore) Close() error {
	store.stopGC()
	return store.db.Close()
}
