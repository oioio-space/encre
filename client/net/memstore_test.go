package net_test

import "github.com/oioio-space/encre/client/net"

// memStore is an in-memory [net.Store] for tests: no browser, no
// filesystem, so the save-every-word and outbox logic is exercised without
// either. saveErr, when set, makes every Save fail the way a full
// localStorage does (see net.ErrStorageFull).
type memStore struct {
	data    map[string][]byte
	saveErr error
}

func newMemStore() *memStore {
	return &memStore{data: map[string][]byte{}}
}

func (m *memStore) Load(key string) ([]byte, error) {
	data, ok := m.data[key]
	if !ok {
		return nil, net.ErrNotFound
	}
	return data, nil
}

func (m *memStore) Save(key string, data []byte) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.data[key] = data
	return nil
}

func (m *memStore) Clear(key string) error {
	delete(m.data, key)
	return nil
}
