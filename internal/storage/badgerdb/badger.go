package badgerdb

import (
	"net/url"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
)

// BadgerStore defines an instance of a badgerdb store, with
// routines for GC and other goodies
type BadgerStore struct {
	BadgerDB *badger.DB
	stopGC   chan struct{}
}

// New returns a store instance backed by badger db.
func New(dsn string, logger badger.Logger) (*BadgerStore, error) {
	path, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	db, err := badger.Open(
		badger.DefaultOptions(path.Path).
			WithLogger(logger).
			WithLoggingLevel(badger.ERROR).
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

// Close close badger DB
func (store *BadgerStore) Close() error {
	close(store.stopGC)
	return store.BadgerDB.Close()
}
