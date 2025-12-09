package storage

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "PENDING"
	StatusRunning   TaskStatus = "RUNNING"
	StatusCompleted TaskStatus = "COMPLETED"
	StatusFailed    TaskStatus = "FAILED"
)

type TaskRecord struct {
	ID        string     `json:"id"`
	Source    string     `json:"source"`
	Status    TaskStatus `json:"status"`
	Result    string     `json:"result"`
	Timestamp int64      `json:"timestamp"`
}

type Store struct {
	db *badger.DB
}

func NewStore(path string) (*Store, error) {
	opts := badger.DefaultOptions(path).
		WithLoggingLevel(badger.ERROR)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) SaveTask(task TaskRecord) error {
	return s.db.Update(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("task:%s", task.ID))

		val, err := json.Marshal(task)
		if err != nil {
			return err
		}

		return txn.Set(key, val)
	})
}

func (s *Store) GetTask(id string) (*TaskRecord, error) {
	var task TaskRecord
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("task:%s", id)))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &task)
		})
	})
	return &task, err
}

func (s *Store) ListTasks() ([]TaskRecord, error) {
	var tasks []TaskRecord
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("task:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var t TaskRecord
				if err := json.Unmarshal(val, &t); err != nil {
					return err
				}
				tasks = append(tasks, t)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return tasks, err
}
