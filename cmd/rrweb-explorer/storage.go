package main

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/pkg/errors"
)

// ErrMessageNotFound is na error returned when a message with a given offset doesn't exist.
var ErrMessageNotFound = errors.New("message not found")

// ConsumerOffset represents consumer offset.
type ConsumerOffset int8

// NewBadgerStore returns a store instance backed by badger db.
func NewBadgerStore(path string, maxCacheSize int64) (*BadgerStore, error) {
	if maxCacheSize == 0 {
		maxCacheSize = 1 << 30 // 1 GB = Badger default
	}
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

	store := &BadgerStore{
		BadgerDB: db,
		stopGC:   make(chan struct{}),
	}
	store.startGC()

	return store, nil
}

type BadgerStore struct {
	BadgerDB *badger.DB
	stopGC   chan struct{}
}

func (store *BadgerStore) Recordings() (*recordingStore, error) {
	return &recordingStore{
		db: store.BadgerDB,
	}, nil
}

func (store *BadgerStore) startGC() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-store.stopGC:
			case <-ticker.C:
			again:
				err := store.BadgerDB.RunValueLogGC(0.5)
				if err == nil {
					goto again
				}
			}
		}
	}()
}

func (store *BadgerStore) Close() error {
	close(store.stopGC)
	return store.BadgerDB.Close()
}
