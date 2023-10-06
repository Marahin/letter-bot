package store

import (
	"sync"
)

type Store[T any] struct {
	lock *sync.RWMutex
	data map[string]T
}

// Creates new fnv-based key/value in-memory store
func NewStore[T any]() *Store[T] {
	return &Store[T]{
		lock: &sync.RWMutex{},
		data: make(map[string]T),
	}
}

func (s *Store[T]) Store(key string, value T) error {
	// Get a nice hash
	keyHash, err := Hash(key)
	if err != nil {
		return err
	}

	s.Lock()
	defer s.Unlock()

	s.data[keyHash] = value

	return nil
}

func (s *Store[T]) Get(key string) T {
	var v T
	keyHash, err := Hash(key)
	if err != nil {
		return v
	}

	s.RLock()
	defer s.RUnlock()
	if res, ok := s.data[keyHash]; ok {
		return res
	}

	return v
}

func (s *Store[T]) List() []string {
	s.RLock()
	defer s.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

func (s *Store[T]) Delete(key string) {
	s.Lock()
	defer s.Unlock()

	delete(s.data, key)
}

func (s *Store[T]) Lock() {
	s.lock.Lock()
}

func (s *Store[T]) Unlock() {
	s.lock.Unlock()
}

func (s *Store[T]) RLock() {
	s.lock.RLock()
}

func (s *Store[T]) RUnlock() {
	s.lock.RUnlock()
}

func Hash(v string) (string, error) {
	return v, nil
}
