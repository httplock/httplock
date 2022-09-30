package storage

import (
	"encoding/json"
	"errors"
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

// EntryHash returns the hash of a path entry
func (r *Root) EntryHash(path []string) (string, error) {
	dCur, err := r.getDir(path[:len(path)-1], false)
	if err != nil {
		return "", err
	}
	name := path[len(path)-1]
	entry, ok := dCur.Entries[name]
	if !ok {
		return "", fmt.Errorf("not found: %s", strings.Join(path, "/"))
	}
	if entry.Hash == "" {
		switch entry.Kind {
		case KindDir:
			err = r.hashDir(entry.dir)
			if err != nil {
				return "", fmt.Errorf("failed to hash directory: %w", err)
			}
			entry.Hash = entry.dir.hash
		case KindFile:
			err = r.hashFile(entry.file)
			if err != nil {
				return "", fmt.Errorf("failed to hash file: %w", err)
			}
			entry.Hash = entry.file.hash
		}
	}
	return entry.Hash, nil
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

func (d *Dir) keys() []string {
	if d == nil || d.Entries == nil {
		return nil
	}
	keys := make([]string, 0, len(d.Entries))
	for k := range d.Entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type DiffReport struct {
	R1      string      `json:"r1"` // hash of root 1
	R2      string      `json:"r2"` // hash of root 2
	Entries []DiffEntry `json:"entries"`
}
type DiffEntry struct {
	Action string   `json:"action"`          // add, remove, changed
	Path   []string `json:"path"`            // path to entry
	Hash1  string   `json:"hash1,omitempty"` // hash of entry from 1
	Hash2  string   `json:"hash2,omitempty"` // hash of entry from 2
}

// DiffRoots compares two roots and returns a list of differences
// TODO: improve entry to show more than the hash and to package request/response into a single entry?
func DiffRoots(r1, r2 *Root) (DiffReport, error) {
	dr := DiffReport{
		Entries: []DiffEntry{},
	}

	if !r1.readonly && r1.dir != nil {
		err := r1.hashDir(r1.dir)
		if err != nil {
			return dr, fmt.Errorf("failed to compute hash on r1: %w", err)
		}
		r1.hash = r1.dir.hash
	}
	if r1.hash == "" {
		return dr, fmt.Errorf("hash missing from r1")
	}
	dr.R1 = r1.hash
	if !r2.readonly && r2.dir != nil {
		err := r2.hashDir(r2.dir)
		if err != nil {
			return dr, fmt.Errorf("failed to compute hash on r2: %w", err)
		}
		r2.hash = r2.dir.hash
	}
	if r2.hash == "" {
		return dr, fmt.Errorf("hash missing from r2")
	}
	dr.R2 = r2.hash

	// setup iterator on each root
	di1, err := newDiffIter(r1)
	if err != nil {
		return dr, err
	}
	di2, err := newDiffIter(r2)
	if err != nil {
		return dr, err
	}
	for {
		// copy slices to be local to loop
		p1 := make([]string, len(di1.curPath))
		p2 := make([]string, len(di2.curPath))
		copy(p1, di1.curPath)
		copy(p2, di2.curPath)
		// compare current iterator
		cmp := cmpPaths(p1, p2)
		if len(p1) > 0 && (len(p2) == 0 || cmp < 0) {
			// deleted
			dr.Entries = append(dr.Entries, DiffEntry{
				Action: "deleted",
				Path:   p1,
				Hash1:  di1.curFileHash,
			})

			_, err = di1.next()
			if err != nil && !errors.Is(err, io.EOF) {
				return dr, err
			}
		} else if len(p2) > 0 && (len(p1) == 0 || cmp > 0) {
			// added
			dr.Entries = append(dr.Entries, DiffEntry{
				Action: "added",
				Path:   p2,
				Hash2:  di2.curFileHash,
			})

			_, err = di2.next()
			if err != nil && !errors.Is(err, io.EOF) {
				return dr, err
			}
		} else {
			// if r1 == r2, both are request headers, but responses differ, show changed
			if len(p1) > 0 {
				if di1.curFileHash != di2.curFileHash {
					dr.Entries = append(dr.Entries, DiffEntry{
						Action: "changed",
						Path:   p1,
						Hash1:  di1.curFileHash,
						Hash2:  di2.curFileHash,
					})
				}
			}

			// iterate
			_, err1 := di1.next()
			if err1 != nil && !errors.Is(err1, io.EOF) {
				return dr, err1
			}
			_, err2 := di2.next()
			if err2 != nil && !errors.Is(err2, io.EOF) {
				return dr, err2
			}
			if err1 != nil && err2 != nil {
				// both list at EOF
				break
			}
		}
	}

	return dr, nil
}

type diffIter struct {
	r                *Root
	curDirs          []*Dir
	remainingEntries [][]string
	curPath          []string
	curFileHash      string
}

func newDiffIter(r *Root) (*diffIter, error) {
	err := r.loadRoot()
	if err != nil {
		return nil, err
	}
	di := diffIter{
		r:                r,
		curDirs:          []*Dir{r.dir},
		remainingEntries: [][]string{r.dir.keys()},
		curPath:          []string{""},
	}
	return &di, nil
}
func (di *diffIter) next() ([]string, error) {
	for {
		// go to the last index of remainingEntries
		i := len(di.remainingEntries) - 1
		if i < 0 {
			return nil, io.EOF
		}
		// if leaf is empty, remove leaf and rerun on parent
		if len(di.remainingEntries[i]) == 0 {
			di.curDirs = di.curDirs[:i]
			di.remainingEntries = di.remainingEntries[:i]
			di.curPath = di.curPath[:i]
			di.curFileHash = ""
			continue
		}
		// move next remaining entry to curPath
		di.curPath[i] = di.remainingEntries[i][0]
		di.remainingEntries[i] = di.remainingEntries[i][1:]
		// handle current entry
		de, ok := di.curDirs[i].Entries[di.curPath[i]]
		if !ok {
			return nil, fmt.Errorf("failed looking up entry: %v", di.curPath)
		}
		if de.Kind == KindFile {
			// if entry is a file, save hash and break (this is the next iter)
			di.curFileHash = de.Hash
			break
		} else if de.Kind == KindDir {
			// if entry is a directory, add a new leaf to process
			var err error
			de.dir, err = di.r.loadDir(de.Hash)
			if err != nil {
				return nil, fmt.Errorf("failed loading dir at %v: %w", di.curPath, err)
			}
			di.curDirs = append(di.curDirs, de.dir)
			di.remainingEntries = append(di.remainingEntries, de.dir.keys())
			di.curPath = append(di.curPath, "")
			di.curFileHash = ""
		} else {
			return nil, fmt.Errorf("unknown entry type %v at %v", de.Kind, di.curPath)
		}
	}

	return di.curPath, nil
}

// cmpPaths returns 0: p1 == p2, +1: p1 > p2, or -1: p1 < p2
func cmpPaths(p1, p2 []string) int {
	for i := 0; i < len(p1); i++ {
		if len(p2) < i+1 {
			// p1 > p2
			return 1
		}
		cmp := strings.Compare(p1[i], p2[i])
		if cmp != 0 {
			return cmp
		}
	}
	if len(p2) > len(p1) {
		// p1 < p2
		return -1
	}
	return 0
}
