package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/httplock/httplock/internal/config"
)

// Filesystem storage is backed by a directory

const fsTmpDir = "tmp"

func init() {
	Register("filesystem", func(c config.Config) (Storage, error) {
		return NewFilesystem(c.Storage.Directory)
	})
}

type FSStorage struct {
	mu    sync.Mutex
	dir   string
	index Index
	roots map[string]*Root
}

func NewFilesystem(dir string) (Storage, error) {
	fi, err := os.Stat(filepath.Join(dir, fsTmpDir))
	if err != nil {
		// create the directory if it doesn't exist
		if errors.Is(err, fs.ErrNotExist) {
			err = os.MkdirAll(filepath.Join(dir, fsTmpDir), 0777)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if !fi.IsDir() {
		return nil, fmt.Errorf("%s exists and is not a directory%.0w", dir, fs.ErrExist)
	}
	return &FSStorage{
		dir:   dir,
		index: readIndex(dir),
		roots: map[string]*Root{},
	}, nil
}
func readIndex(dir string) Index {
	ind := Index{
		Roots: map[string]*IndexRoot{},
	}
	fh, err := os.Open(filepath.Join(dir, filenameIndexJSON))
	if err != nil {
		return ind
	}
	defer fh.Close()
	indBytes, err := io.ReadAll(fh)
	if err != nil {
		return ind
	}
	// same result regardless of whether the unmarshal succeeds
	_ = json.Unmarshal(indBytes, &ind)
	return ind
}

// BlobOpen returns a reader for a blob
func (fs *FSStorage) BlobOpen(blob string) (BlobReader, error) {
	fh, err := os.OpenFile(filepath.Join(fs.dir, blob), os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	stat, err := fh.Stat()
	if err != nil {
		return nil, err
	}
	return newBlobReader(fh, stat.Size())
}

// BlobCreate returns a writer for a blob
func (fs *FSStorage) BlobCreate() (BlobWriter, error) {
	fh, err := os.CreateTemp(filepath.Join(fs.dir, fsTmpDir), "*")
	if err != nil {
		return nil, err
	}
	return newBlobWriter(fh, func(hash string) error {
		err := os.Rename(fh.Name(), filepath.Join(fs.dir, hash))
		if err != nil {
			os.Remove(fh.Name())
			return err
		}
		return nil
	}), nil
}

// Flush writes any data cached to the backend storage
func (fs *FSStorage) Flush() error {
	// noop, hashes are written to disk on Save and uuid's are not flushed
	return nil
}

// Index returns the current index
func (fs *FSStorage) Index() Index {
	return fs.index
}

// PruneCache deletes any data from memory or cache that hasn't been recently accessed
func (fs *FSStorage) PruneCache(time.Duration) error {
	return errNotImplemented
}

// PruneStorage deletes any blobs that are not used by any root
func (fs *FSStorage) PruneStorage() error {
	return errNotImplemented
}

// RootCreate returns a new root using a uuid
func (fs *FSStorage) RootCreate() (string, *Root, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	u := fmt.Sprintf("uuid:%s", uuid.New().String())
	root := newRoot(fs)
	fs.roots[u] = root
	return u, root, nil
}

// RootCreateFrom returns a new root using a uuid initialized from an existing hash
func (fs *FSStorage) RootCreateFrom(hash string) (string, *Root, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	u := fmt.Sprintf("uuid:%s", uuid.New().String())
	if _, ok := fs.index.Roots[hash]; !ok {
		return "", nil, fmt.Errorf("hash not found in index: %s", hash)
	}
	root := newRootHash(fs, hash)
	root.readonly = false
	fs.roots[u] = root
	return u, root, nil
}

// RootOpen returns an existing root
func (fs *FSStorage) RootOpen(name string) (*Root, error) {
	if root, ok := fs.roots[name]; ok {
		return root, nil
	}
	if _, ok := fs.index.Roots[name]; !ok {
		return nil, fmt.Errorf("hash not found in index: %s", name)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.index.Roots[name].Used = time.Now() // TODO: consider moving up
	root := newRootHash(fs, name)
	fs.roots[name] = root
	return root, nil
}

// RootSave saves a root and adds the hash to the index
func (fs *FSStorage) RootSave(r *Root) (string, error) {
	hash, err := r.Save()
	if err != nil {
		return "", err
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	newRoot := newRootHash(fs, hash)
	fs.roots[hash] = newRoot
	fs.index.Roots[hash] = &IndexRoot{
		Used: time.Now(),
	}
	err = fs.writeIndex()
	if err != nil {
		return "", err
	}
	return hash, nil
}

func (fs *FSStorage) writeIndex() error {
	indBytes, err := json.Marshal(fs.index)
	if err != nil {
		return err
	}
	fh, err := os.Create(filepath.Join(fs.dir, filenameIndexJSON))
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fh.Write(indBytes)
	if err != nil {
		return err
	}
	return nil
}
