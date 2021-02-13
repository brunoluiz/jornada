package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v2"
)

const (
	recordingStoreNamespace = "recording"
)

type recordingStore struct {
	db *badger.DB
}

// Client keeps the recording client information, mostly parsed from .UserAgent
type Client struct {
	UserAgent string `json:"userAgent"`
	OS        string `json:"os"`
	Browser   string `json:"browser"`
	Version   string `json:"version"`
}

// Record recording record model, mostly with data from the events, user and browser used
type Record struct {
	ID   string            `json:"id"`
	Meta map[string]string `json:"meta"`
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Client    Client    `json:"client"`
	ClientID  string    `json:"clientId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (store *recordingStore) Save(ctx context.Context, rec Record) error {
	return store.db.Update(func(tx *badger.Txn) error {
		rec.UpdatedAt = time.Now()

		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}

		return tx.Set([]byte(store.key(rec.ID)), data)
	})
}

func (store *recordingStore) GetByID(ctx context.Context, id string) (Record, error) {
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

func (store *recordingStore) GetAll(ctx context.Context, offset string, limit int) ([]Record, error) {
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

func (store *recordingStore) key(id string) []byte {
	return []byte(recordingStoreNamespace + "/" + id)
}
