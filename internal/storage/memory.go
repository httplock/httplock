package storage

import (
	"bytes"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/httplock/httplock/internal/config"
)

// Memory storage only exists in memory and is lost when the data structure is dereferenced

func init() {
	Register("memory", func(c config.Config) (Storage, error) {
		return NewMemory()
	})
}

type MemStorage struct {
	mu    sync.Mutex
	index Index
	roots map[string]*Root
	blobs map[string][]byte
}

func NewMemory() (Storage, error) {
	return &MemStorage{
		index: Index{
			Roots: map[string]*IndexRoot{},
		},
		roots: map[string]*Root{},
		blobs: map[string][]byte{},
	}, nil
}

// BlobOpen returns a reader for a blob
func (m *MemStorage) BlobOpen(blob string) (BlobReader, error) {
	if bb, ok := m.blobs[blob]; ok {
		br := bytes.NewReader(bb)
		return newBlobReader(br, int64(len(bb)))
	}
	return nil, fs.ErrNotExist
}

// BlobCreate returns a writer for a blob
func (m *MemStorage) BlobCreate() (BlobWriter, error) {
	b := bytes.Buffer{}
	bw := newBlobWriter(&b, func(hash string) error {
		m.blobs[hash] = b.Bytes()
		return nil
	})
	return bw, nil
}

// Flush writes any data cached to the backend storage
func (m *MemStorage) Flush() error {
	// noop, all data is in memory, never written to disk
	return nil
}

// Index returns the current index
func (m *MemStorage) Index() Index {
	return m.index
}

// PruneCache deletes any data from memory or cache that hasn't been recently accessed
func (m *MemStorage) PruneCache(time.Duration) error {
	return errNotImplemented
}

// PruneStorage deletes any blobs that are not used by any root
func (m *MemStorage) PruneStorage() error {
	return errNotImplemented
}

// RootCreate returns a new root using a uuid
func (m *MemStorage) RootCreate() (string, *Root, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := fmt.Sprintf("uuid:%s", uuid.New().String())
	root := newRoot(m)
	m.roots[u] = root
	return u, root, nil
}

// RootCreateFrom returns a new root using a uuid initialized from an existing hash
func (m *MemStorage) RootCreateFrom(hash string) (string, *Root, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := fmt.Sprintf("uuid:%s", uuid.New().String())
	if _, ok := m.index.Roots[hash]; !ok {
		return "", nil, fmt.Errorf("hash not found in index: %s", hash)
	}
	root := newRootHash(m, hash)
	root.readonly = false
	m.roots[u] = root
	return u, root, nil
}

// RootOpen returns an existing root
func (m *MemStorage) RootOpen(name string) (*Root, error) {
	if root, ok := m.roots[name]; ok {
		return root, nil
	}
	if _, ok := m.index.Roots[name]; !ok {
		return nil, fmt.Errorf("hash not found in index: %s", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.index.Roots[name].Used = time.Now()
	root := newRootHash(m, name)
	m.roots[name] = root
	return root, nil
}

// RootSave saves a root and adds the hash to the index
func (m *MemStorage) RootSave(r *Root) (string, error) {
	hash, err := r.Save()
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	newRoot := newRootHash(m, hash)
	m.roots[hash] = newRoot
	m.index.Roots[hash] = &IndexRoot{
		Used: time.Now(),
	}
	return hash, nil
}
