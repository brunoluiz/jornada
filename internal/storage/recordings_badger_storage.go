package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v2"
)

const (
	recordingStoreNamespace = "recording"
)

type recordingStoreBadger struct {
	db *badger.DB
}

func (store *recordingStoreBadger) Save(ctx context.Context, rec Record) error {
	return store.db.Update(func(tx *badger.Txn) error {
		rec.UpdatedAt = time.Now()

		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		return tx.Set([]byte(store.key(rec.ID)), data)
	})
}

func (store *recordingStoreBadger) GetByID(ctx context.Context, id string) (Record, error) {
	var rec Record

	err := store.db.View(func(tx *badger.Txn) error {
		item, err := tx.Get(store.key(id))
		if err != nil {
			return err
		}

		return item.Value(func(b []byte) error {
			return json.Unmarshal(b, &rec)
		})
	})

	return rec, err
}

func (store *recordingStoreBadger) GetAll(ctx context.Context, offset string, limit int) ([]Record, error) {
	records := []Record{}

	err := store.db.View(func(tx *badger.Txn) error {
		it := tx.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Reverse:        false,
			AllVersions:    false,
		})
		defer it.Close()

		for it.Seek(store.key(offset)); it.ValidForPrefix([]byte(store.key(""))) && len(records) < limit; it.Next() {
			err := it.Item().Value(func(b []byte) error {
				var rec Record
				if err := json.Unmarshal(b, &rec); err != nil {
					return err
				}

				records = append(records, rec)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return records, err
}

func (store *recordingStoreBadger) key(id string) []byte {
	return []byte(recordingStoreNamespace + "/" + id)
}
