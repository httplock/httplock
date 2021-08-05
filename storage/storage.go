package storage

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

type Storage struct {
	Backing backing.Backing
	roots   map[string]*Dir
}

func New(c config.Config) (*Storage, error) {
	b := backing.Get(c)
	if b == nil {
		return nil, fmt.Errorf("Backing not found: %s", c.Storage.Backing)
	}
	s := Storage{
		Backing: b,
		roots:   map[string]*Dir{},
	}
	// TODO: load the index.json if it exists
	return &s, nil
}

func (s *Storage) NewRoot() (string, *Dir, error) {
	u := fmt.Sprintf("uuid:%s", uuid.New().String())
	d, err := DirNew(s.Backing)
	if err != nil {
		return "", nil, err
	}
	s.roots[u] = d
	return u, d, nil
}

func (s *Storage) GetRoot(name string) (*Dir, error) {
	d, ok := s.roots[name]
	if ok && d != nil {
		return d, nil
	}
	d, err := DirLoad(s.Backing, name)
	if err == nil {
		return d, nil
	}
	return nil, fmt.Errorf("Root not found: %s", name)
}

func (s *Storage) ListRoots() ([]string, error) {
	keys := make([]string, 0, len(s.roots))
	for k := range s.roots {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Storage) SaveRoot(name string) (string, error) {
	d, ok := s.roots[name]
	if !ok {
		return "", fmt.Errorf("Root not found: %s", name)
	}
	h, err := d.Hash()
	if err != nil {
		return "", fmt.Errorf("Hashing root failed: %w", err)
	}
	s.roots[h] = d
	// TODO: add entry to index.json
	return h, nil
}
