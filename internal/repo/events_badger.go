package repo

import (
	"context"
	"encoding/binary"
	"math"

	"github.com/dgraph-io/badger/v2"
)

// EventBadgerV2 defines an event storage using badger v2
// This storage is using the following format: events/{session_id}/{event_sequential_id}
// Each new event sent by the recording library is going to have a sequential ID, making it
// easy to seek afterwards
type EventBadgerV2 struct {
	db *badger.DB
}

// NewEventBadger returns a new *EventBadgerV2
func NewEventBadger(db *badger.DB) *EventBadgerV2 {
	return &EventBadgerV2{db}
}

// Add bulk adds events for a certain session id -- key value will be suffixed with sequential ID
func (store *EventBadgerV2) Add(ctx context.Context, sessionID string, msgs ...[]byte) error {
	return store.db.Update(func(tx *badger.Txn) error {
		seq, err := store.lastSequence(tx, sessionID)
		if err != nil {
			return err
		}

		for _, msg := range msgs {
			seq++
			if err := store.writeMsg(tx, sessionID, seq, msg); err != nil {
				return err
			}
		}

		return nil
	})
}

// Get all events for a certain session id
func (store *EventBadgerV2) Get(ctx context.Context, sessionID string, cb func(b []byte, pos, size uint64) error) error {
	return store.db.View(func(tx *badger.Txn) error {
		it := tx.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Reverse:        false,
			AllVersions:    false,
		})
		defer it.Close()

		lastID, err := store.lastSequence(tx, sessionID)
		if err != nil {
			return err
		}

		var count uint64
		for it.Seek(store.messageKey(sessionID, 1)); it.ValidForPrefix([]byte(store.id(sessionID))); it.Next() {
			if err := it.Item().Value(func(msg []byte) error {
				if err := cb(msg, count, lastID); err != nil {
					return err
				}
				count++
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// lastSequence gets last ID saved in DB
func (store *EventBadgerV2) lastSequence(tx *badger.Txn, id string) (uint64, error) {
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

func (store *EventBadgerV2) writeMsg(tx *badger.Txn, id string, seq uint64, msg []byte) error {
	return tx.Set(store.messageKey(id, seq), msg)
}

func (store *EventBadgerV2) messageKey(id string, seq uint64) []byte {
	key := make([]byte, len(store.id(id))+8)
	copy(key, store.id(id))
	binary.BigEndian.PutUint64(key[len(store.id(id)):], seq)

	return key
}

func (store *EventBadgerV2) id(id string) string {
	return "events/" + id + "/"
}
