package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"

	"github.com/httplock/httplock/hasher"
	"github.com/httplock/httplock/internal/storage/backing"
)

type entryType int

const (
	entryDir entryType = iota
	entryFile
	entryComplex
)

type Dir struct {
	mu      sync.Mutex
	hash    string
	backing backing.Backing
	Entries map[string]*DirEntry
	// // TODO: hash on child dir entries needed, but self hash can't exist when computing hash
	// // should hash be moved into a separate object that points to a directory or file
	// hash  string
	// Dirs  map[string]*dir  `json:"dirs,omitempty"`
	// Files map[string]*file `json:"files,omitempty"`
}

type DirEntry struct {
	Hash    string    `json:"hash"`
	Kind    entryType `json:"kind"`
	dir     *Dir
	file    *File
	complex *ComplexFile
}

func (e entryType) MarshalText() ([]byte, error) {
	var s string
	switch e {
	default:
		s = ""
	case entryDir:
		s = "dir"
	case entryFile:
		s = "file"
	case entryComplex:
		s = "complex"
	}
	return []byte(s), nil
}

func (e *entryType) UnmarshalText(b []byte) error {
	switch strings.ToLower(string(b)) {
	default:
		return fmt.Errorf("unknown entry type \"%s\"", b)
	case "dir":
		*e = entryDir
	case "file":
		*e = entryFile
	case "complex":
		*e = entryComplex
	}
	return nil
}

func DirNew(backing backing.Backing) (*Dir, error) {
	d := Dir{
		mu:      sync.Mutex{},
		backing: backing,
		Entries: map[string]*DirEntry{},
	}
	return &d, nil
}

func DirLoad(backing backing.Backing, hash string) (*Dir, error) {
	var d Dir
	rdr, err := backing.Read(hash)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(rdr)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &d)
	if err != nil {
		return nil, err
	}
	d.backing = backing
	d.hash = hash
	d.mu = sync.Mutex{}
	return &d, nil
}

func (d *Dir) CreateDir(name string) (*Dir, error) {
	if _, ok := d.Entries[name]; ok {
		return nil, fmt.Errorf("directory entry exists with same name \"%s\"", name)
	}
	newDir, err := DirNew(d.backing)
	if err != nil {
		return nil, err
	}
	de := DirEntry{
		Kind: entryDir,
		dir:  newDir,
	}
	d.mu.Lock()
	d.Entries[name] = &de
	d.mu.Unlock()
	return newDir, nil
}

func (d *Dir) CreateFile(name string) (*File, error) {
	if _, ok := d.Entries[name]; ok {
		return nil, fmt.Errorf("directory entry exists with same name \"%s\"", name)
	}
	newFile, err := FileNew(d.backing)
	if err != nil {
		return nil, err
	}
	de := DirEntry{
		Kind: entryFile,
		file: newFile,
	}
	d.mu.Lock()
	d.Entries[name] = &de
	d.mu.Unlock()
	return newFile, nil
}

func (d *Dir) CreateComplex(name string) (*ComplexFile, error) {
	if _, ok := d.Entries[name]; ok {
		return nil, fmt.Errorf("directory entry exists with same name \"%s\"", name)
	}
	newComplex, err := ComplexNew(d.backing)
	if err != nil {
		return nil, err
	}
	de := DirEntry{
		Kind:    entryComplex,
		complex: newComplex,
	}
	d.mu.Lock()
	d.Entries[name] = &de
	d.mu.Unlock()
	return newComplex, nil
}

func (d *Dir) GetDir(name string) (*Dir, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	de, ok := d.Entries[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if de.Kind != entryDir {
		return nil, fmt.Errorf("entry is not a directory")
	}
	if de.dir == nil {
		ded, err := DirLoad(d.backing, de.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to load directory")
		}
		d.Entries[name].dir = ded
	}
	return d.Entries[name].dir, nil
}

func (d *Dir) GetComplex(name string) (*ComplexFile, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	de, ok := d.Entries[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if de.Kind != entryComplex {
		return nil, fmt.Errorf("entry is not a complex file")
	}
	if de.complex == nil {
		dec, err := ComplexLoad(d.backing, de.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to load complex file")
		}
		d.Entries[name].complex = dec
	}
	return d.Entries[name].complex, nil
}

func (d *Dir) GetFile(name string) (*File, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	de, ok := d.Entries[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if de.Kind != entryFile {
		return nil, fmt.Errorf("entry is not a file")
	}
	if de.file == nil {
		def, err := FileLoad(d.backing, de.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to load file")
		}
		d.Entries[name].file = def
	}
	return d.Entries[name].file, nil
}

func (d *Dir) Hash() (string, error) {
	// TODO: hash can be returned directly if no writes have occurred within the dir
	// first hash all directory entries
	for key := range d.Entries {
		switch d.Entries[key].Kind {
		case entryDir:
			if d.Entries[key].dir != nil {
				h, err := d.Entries[key].dir.Hash()
				if err != nil {
					return "", err
				}
				d.Entries[key].Hash = h
			}
		case entryComplex:
			if d.Entries[key].complex != nil {
				h, err := d.Entries[key].complex.Hash()
				if err != nil {
					return "", err
				}
				d.Entries[key].Hash = h
			}
		case entryFile:
			if d.Entries[key].file != nil {
				h, err := d.Entries[key].file.Hash()
				if err != nil {
					return "", err
				}
				d.Entries[key].Hash = h
			}
		default:
			return "", fmt.Errorf("unknown entry type: %d", d.Entries[key].Kind)
		}
	}
	// serialize the directory into a byte stream, hash, and save to storage
	b, err := json.Marshal(d)
	if err != nil {
		return "", err
	}
	hash, err := hasher.FromBytes(b)
	if err != nil {
		return "", err
	}
	// TODO: ignore errors if file already exists?
	wd, err := d.backing.Write(hash)
	if err != nil {
		return "", err
	}
	defer wd.Close()
	_, err = wd.Write(b)
	if err != nil {
		return "", err
	}
	// save and return the hash
	d.hash = hash
	return hash, nil
}

// Raw returns the marshaled version of the directory
func (d *Dir) Raw() []byte {
	// TODO: store and reuse raw content
	b, _ := json.Marshal(d)
	return b
}

// TODO:
// - d.GetDir(name) (dir, error): get subdir, pulling from backend storage if not already in memory
// - d.GetFile(name) (file, error): get file, creating new object if it doesn't already exist
// - ??? d.Serialize() ([]byte, error): convert directory into serial form, triggers hash on all child objects
// - d.Walk(fnDir, fnFile)
// - f.MarshalJSON(): update hash if needed, and then convert struct to json
