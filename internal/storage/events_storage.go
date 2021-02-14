package storage

import (
	"context"
	"encoding/binary"
	"math"

	"github.com/dgraph-io/badger/v2"
)

const (
	eventsStoreNamespace = "events"
)

type EventStore struct {
	db *badger.DB
}

func NewEventStoreBadger(db *badger.DB) *EventStore {
	return &EventStore{db}
}

func (store *EventStore) Add(ctx context.Context, id string, msgs ...[]byte) error {
	return store.db.Update(func(tx *badger.Txn) error {
		seq, err := store.lastSequence(tx, id)
		if err != nil {
			return err
		}

		for _, msg := range msgs {
			seq++
			if err := store.writeMsg(tx, id, seq, msg); err != nil {
				return err
			}
		}

		return nil
	})
}

func (store *EventStore) Get(ctx context.Context, id string, cb func(b []byte, last bool) error) error {
	return store.db.View(func(tx *badger.Txn) error {
		it := tx.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Reverse:        false,
			AllVersions:    false,
		})
		defer it.Close()

		lastID, err := store.lastSequence(tx, id)
		if err != nil {
			return err
		}

		for it.Seek(store.messageKey(id, 1)); it.ValidForPrefix([]byte(store.id(id))); it.Next() {
			if err := it.Item().Value(func(msg []byte) error {
				return cb(msg, string(it.Item().Key()) == string(store.messageKey(id, lastID)))
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (store *EventStore) lastSequence(tx *badger.Txn, id string) (uint64, error) {
	it := tx.NewIterator(badger.IteratorOptions{
		PrefetchValues: false,
		Reverse:        true,
	})
	defer it.Close()

	// if no data is available, initialise storage for this recording
	if it.Seek(store.messageKey(id, math.MaxUint64)); !it.ValidForPrefix([]byte(store.id(id))) {
		if err := store.writeMsg(tx, id, 0, []byte{}); err != nil {
			return 0, err
		}
		return store.lastSequence(tx, id)
	}

	lastKey := it.Item().Key()
	return binary.BigEndian.Uint64(lastKey[len(store.id(id)):]), nil
}

func (store *EventStore) writeMsg(tx *badger.Txn, id string, seq uint64, msg []byte) error {
	return tx.Set(store.messageKey(id, seq), msg)
}

func (store *EventStore) messageKey(id string, seq uint64) []byte {
	key := make([]byte, len(store.id(id))+8)
	copy(key, store.id(id))
	binary.BigEndian.PutUint64(key[len(store.id(id)):], seq)

	return key
}

func (store *EventStore) id(id string) string {
	return eventsStoreNamespace + "/" + id + "/"
}
