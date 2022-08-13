// Package storage implements the various backend storage types
package storage

import (
	"errors"
	"fmt"
	"time"

	"github.com/httplock/httplock/internal/config"
)

var errNotImplemented = errors.New("not implemented")

const (
	filenameIndexJSON = "index.json"
	filenameIndexMD   = "index.md"
	filenameHTTPLock  = "httplock"
)

type fileHTTPLock struct {
	Version string `json:"httplockVersion"`
}

// Storage is implemented by various backings (memory, disk, OCI registry)
type Storage interface {
	// BlobOpen returns a reader for a blob
	BlobOpen(blob string) (BlobReader, error)
	// BlobCreate returns a writer for a blob
	BlobCreate() (BlobWriter, error)
	// Flush writes any data cached to the backend storage
	Flush() error
	// Index returns the current index
	Index() Index
	// PruneCache deletes any data from memory or cache that hasn't been recently accessed
	PruneCache(time.Duration) error
	// PruneStorage deletes any blobs that are not used by any root
	PruneStorage() error
	// RootCreate returns a new root using a uuid
	RootCreate() (string, *Root, error)
	// RootCreateFrom returns a new root using a uuid initialized from an existing hash
	RootCreateFrom(hash string) (string, *Root, error)
	// RootOpen returns an existing root
	RootOpen(name string) (*Root, error)
	// RootSave saves a root and adds the hash to the index
	RootSave(r *Root) (string, error)
}

// Index lists the known roots stored by hash
type Index struct {
	Roots map[string]*IndexRoot `json:"roots"`
}

// IndexRoot describes the metadata for a root
type IndexRoot struct {
	Used time.Time `json:"used,omitempty"`
}

var registered = map[string]func(config.Config) (Storage, error){}

func Register(name string, s func(config.Config) (Storage, error)) {
	registered[name] = s
}

func Get(c config.Config) (Storage, error) {
	if fn, ok := registered[c.Storage.Kind]; ok {
		return fn(c)
	}
	return nil, fmt.Errorf("storage kind %s not supported", c.Storage.Kind)
}
