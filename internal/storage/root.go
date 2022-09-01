package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type Root struct {
	storage  Storage
	hash     string
	dir      *Dir
	readonly bool
}

type Dir struct {
	mu      sync.Mutex
	hash    string
	Entries map[string]*DirEntry `json:"entries"`
}

type File struct {
	hash  string
	blobW BlobWriter
}

type EntryKind int

const (
	KindDir EntryKind = iota
	KindFile
)

type DirEntry struct {
	Hash string    `json:"hash"`
	Kind EntryKind `json:"kind"`
	dir  *Dir
	file *File
}

func newRoot(s Storage) *Root {
	return &Root{
		storage: s,
		dir: &Dir{
			Entries: map[string]*DirEntry{},
		},
	}
}
func newRootHash(s Storage, hash string) *Root {
	return &Root{
		storage:  s,
		hash:     hash,
		readonly: true,
	}
}

// Link references an existing blob as a file in a directory
func (r *Root) Link(path []string, blob string) error {
	dCur, err := r.getDir(path[:len(path)-1], true)
	if err != nil {
		return err
	}
	dCur.mu.Lock()
	defer dCur.mu.Unlock()
	name := path[len(path)-1]
	if entry, ok := dCur.Entries[name]; ok {
		if entry.Kind != KindFile {
			return fmt.Errorf("%s exists and is not a file", strings.Join(path, "/"))
		}
	}
	// create a new file
	file := &File{hash: blob}
	dCur.Entries[name] = &DirEntry{
		Kind: KindFile,
		file: file,
	}
	return nil
}

// List returns the directory entries in a root
func (r *Root) List(path []string) (map[string]*DirEntry, error) {
	dCur, err := r.getDir(path, false)
	if err != nil {
		return nil, err
	}
	return dCur.Entries, nil
}

// ListHashes returns a slice of hashes representing all entries in a root
func (r *Root) ListHashes() ([]string, error) {
	err := r.loadRoot()
	if err != nil {
		return nil, err
	}
	hashes := []string{}
	fns := WalkFns{
		fnDir: func(d *Dir) error {
			err := r.hashDir(d)
			if err != nil {
				return err
			}
			hashes = append(hashes, d.hash)
			return nil
		},
		fnFile: func(f *File) error {
			err := r.hashFile(f)
			if err != nil {
				return err
			}
			hashes = append(hashes, f.hash)
			return nil
		},
	}
	err = r.Walk(fns)
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

// Read returns a reader for a given file
func (r *Root) Read(path []string) (BlobReader, error) {
	dCur, err := r.getDir(path[:len(path)-1], false)
	if err != nil {
		return nil, err
	}
	name := path[len(path)-1]
	entry, ok := dCur.Entries[name]
	if !ok {
		return nil, fmt.Errorf("not found: %s", strings.Join(path, "/"))
	}
	if entry.Kind != KindFile {
		return nil, fmt.Errorf("%s exists and is not a file", strings.Join(path, "/"))
	}
	if entry.file != nil {
		err = r.hashFile(entry.file)
		if err != nil {
			return nil, err
		}
		entry.Hash = entry.file.hash
	}
	return r.storage.BlobOpen(entry.Hash)
}

// ReadOnly returns true if the root is read-only (loaded from an immutable hash)
func (r *Root) ReadOnly() bool {
	return r.readonly
}

// Save computes and returns the hash of the root
func (r *Root) Save() (string, error) {
	if !r.readonly && r.dir != nil {
		err := r.hashDir(r.dir)
		if err != nil {
			return "", err
		}
		r.hash = r.dir.hash
	}
	if r.hash == "" {
		return "", fmt.Errorf("hash missing")
	}
	return r.hash, nil
}

type WalkFns struct {
	fnDir  func(*Dir) error
	fnFile func(*File) error
}

func (r *Root) Walk(fns WalkFns) error {
	err := r.loadRoot()
	if err != nil {
		return err
	}
	return r.walkDir(fns, r.dir)
}
func (r *Root) walkDir(fns WalkFns, d *Dir) error {
	err := fns.fnDir(d)
	if err != nil {
		return err
	}
	// TODO: sort by name
	for _, entry := range d.Entries {
		switch entry.Kind {
		case KindDir:
			if entry.dir == nil {
				ld, err := r.loadDir(entry.Hash)
				if err != nil {
					return err
				}
				entry.dir = ld
			}
			// recursive into child directories
			err = r.walkDir(fns, entry.dir)
			if err != nil {
				return err
			}
		case KindFile:
			if entry.file == nil {
				lf, err := r.loadFile(entry.Hash)
				if err != nil {
					return err
				}
				entry.file = lf
			}
			err := fns.fnFile(entry.file)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Root) Write(path []string) (BlobWriter, error) {
	// fail if root is read-only
	if r.readonly {
		return nil, errReadOnly
	}
	dCur, err := r.getDir(path[:len(path)-1], true)
	if err != nil {
		return nil, err
	}
	dCur.mu.Lock()
	defer dCur.mu.Unlock()
	name := path[len(path)-1]
	if entry, ok := dCur.Entries[name]; ok {
		if entry.Kind != KindFile {
			return nil, fmt.Errorf("%s exists and is not a file", strings.Join(path, "/"))
		}
		if entry.file == nil {
			file, err := r.loadFile(entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed to load %s: %w", strings.Join(path, "/"), err)
			}
			entry.file = file
		}
	} else {
		// create a new file
		file := &File{}
		dCur.Entries[name] = &DirEntry{
			Kind: KindFile,
			file: file,
		}
	}
	// create blob writer, update
	bw, err := r.storage.BlobCreate()
	if err != nil {
		return nil, err
	}
	dCur.Entries[name].file.blobW = bw
	return bw, nil
}

func (r *Root) getDir(path []string, write bool) (*Dir, error) {
	// fail if root is read-only
	if r.readonly && write {
		return nil, errReadOnly
	}
	err := r.loadRoot()
	if err != nil {
		return nil, err
	}
	dCur := r.dir
	// clear hashes on a write, they will no longer be valid
	if write {
		r.hash = ""
	}
	for i, name := range path {
		if write {
			dCur.hash = ""
		}
		if entry, ok := dCur.Entries[name]; ok {
			if entry.Kind != KindDir {
				return nil, fmt.Errorf("%s exists and is not a directory", strings.Join(path[:i+1], "/"))
			}
			if entry.dir == nil {
				dir, err := r.loadDir(entry.Hash)
				if err != nil {
					return nil, fmt.Errorf("failed to load %s: %w", strings.Join(path[:i+1], "/"), err)
				}
				entry.dir = dir
			}
			if write {
				entry.Hash = ""
			}
			dCur = entry.dir
		} else if write {
			// create a new directory
			dir := &Dir{
				Entries: map[string]*DirEntry{},
			}
			dCur.mu.Lock()
			dCur.Entries[name] = &DirEntry{
				Kind: KindDir,
				dir:  dir,
			}
			dCur.mu.Unlock()
			dCur = dir
		} else {
			return nil, fmt.Errorf("%s not found", strings.Join(path[:i+1], "/"))
		}
	}
	if write {
		dCur.hash = ""
	}
	return dCur, nil
}

func (r *Root) hashDir(d *Dir) error {
	for name, entry := range d.Entries {
		if entry.Hash == "" {
			// an empty hash requires the dir or file pointer to be defined
			switch entry.Kind {
			case KindDir:
				err := r.hashDir(entry.dir)
				if err != nil {
					return err
				}
				entry.Hash = entry.dir.hash
			case KindFile:
				err := r.hashFile(entry.file)
				if err != nil {
					return fmt.Errorf("hashing file \"%s\" failed: %v", name, err)
				}
				entry.Hash = entry.file.hash
			}
		}
	}
	// marshal dir, push blob, save hash
	dj, err := json.Marshal(d)
	if err != nil {
		return err
	}
	bw, err := r.storage.BlobCreate()
	if err != nil {
		return err
	}
	_, err = bw.Write(dj)
	bw.Close()
	if err != nil {
		return err
	}
	d.hash, err = bw.Hash()
	if err != nil {
		return err
	}
	return nil
}
func (r *Root) hashFile(f *File) error {
	if f.blobW != nil {
		hash, err := f.blobW.Hash()
		if err != nil {
			return err // blob writer is likely open
		}
		f.hash = hash
		f.blobW = nil // once hash is extracted, writer is no longer needed
	}
	if f.hash == "" {
		return fmt.Errorf("file hash missing")
	}
	return nil
}

func (r *Root) loadDir(hash string) (*Dir, error) {
	br, err := r.storage.BlobOpen(hash)
	if err != nil {
		return nil, err
	}
	dj, err := io.ReadAll(br)
	br.Close()
	if err != nil {
		return nil, err
	}
	d := Dir{}
	err = json.Unmarshal(dj, &d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Root) loadFile(hash string) (*File, error) {
	return &File{hash: hash}, nil
}

func (r *Root) loadRoot() error {
	if r.dir == nil {
		dir, err := r.loadDir(r.hash)
		if err != nil {
			return err
		}
		r.dir = dir
	}
	return nil
}

func (r *Root) report() (string, error) {
	var err error
	if r.dir == nil {
		r.dir, err = r.loadDir(r.hash)
		if err != nil {
			return "", err
		}
	}
	lines := []string{
		fmt.Sprintf("[%s](./%s)", r.hash, r.hash),
		"",
	}
	lines, err = r.reportDir(r.dir, "", lines)
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n") + "\n", nil
}
func (r *Root) reportDir(d *Dir, prefix string, lines []string) ([]string, error) {
	var err error
	keys := []string{}
	for name := range d.Entries {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry := d.Entries[name]
		switch entry.Kind {
		case KindDir:
			if entry.dir == nil {
				entry.dir, err = r.loadDir(entry.Hash)
				if err != nil {
					return nil, err
				}
			}
			lines = append(lines, fmt.Sprintf("%s- [%s](./%s)", prefix, name, entry.Hash))
			lines, err = r.reportDir(entry.dir, prefix+"  ", lines)
			if err != nil {
				return nil, err
			}
		case KindFile:
			lines = append(lines, fmt.Sprintf("%s- [%s](./%s)", prefix, name, entry.Hash))
		}
	}
	return lines, nil
}

func (k EntryKind) MarshalText() ([]byte, error) {
	switch k {
	case KindDir:
		return []byte("dir"), nil
	case KindFile:
		return []byte("file"), nil
	}
	return []byte(""), fmt.Errorf("undefined directory entry kind: %d", k)
}

func (k *EntryKind) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	case "dir":
		*k = KindDir
	case "file":
		*k = KindFile
	default:
		return fmt.Errorf("unknown directory entry kind: %s", b)
	}
	return nil
}
