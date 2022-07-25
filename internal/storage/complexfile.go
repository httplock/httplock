package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/httplock/httplock/hasher"
	"github.com/httplock/httplock/internal/storage/backing"
)

type ComplexFile struct {
	mu      sync.Mutex
	hash    string
	Entries map[string]*ComplexEntry `json:"entries"`
	backing backing.Backing
}

type ComplexEntry struct {
	Hash string `json:"hash"`
	f    *File
}

func ComplexNew(backing backing.Backing) (*ComplexFile, error) {
	cf := ComplexFile{
		mu:      sync.Mutex{},
		Entries: map[string]*ComplexEntry{},
		backing: backing,
	}
	return &cf, nil
}

// ComplexLoad reads a complexFile from backend storage
func ComplexLoad(backing backing.Backing, hash string) (*ComplexFile, error) {
	var cf ComplexFile
	rdr, err := backing.Read(hash)
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadAll(rdr)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &cf)
	if err != nil {
		return nil, err
	}
	cf.backing = backing
	cf.hash = hash
	return &cf, nil
}

func (cf *ComplexFile) Delete() error {
	if cf.hash != "" {
		return cf.backing.Delete(cf.hash)
	}
	return fmt.Errorf("complex File has not been written to storage")
}

func (cf *ComplexFile) Read(entry string) (io.ReadCloser, error) {
	e, ok := cf.Entries[entry]
	if !ok {
		return nil, fmt.Errorf("entry not found")
	}
	// load file if needed
	if e.f == nil {
		f, err := FileLoad(cf.backing, e.Hash)
		if err != nil {
			return nil, err
		}
		e.f = f
	}
	return e.f.Read()
}

func (cf *ComplexFile) Write(entry string) (io.WriteCloser, error) {
	if _, ok := cf.Entries[entry]; ok {
		return nil, fmt.Errorf("entry already exists")
	}
	f, err := FileNew(cf.backing)
	if err != nil {
		return nil, err
	}
	e := ComplexEntry{
		f: f,
	}
	cf.mu.Lock()
	cf.Entries[entry] = &e
	cf.mu.Unlock()
	return f.Write()
}

func (cf *ComplexFile) Hash() (string, error) {
	// first gather hash on each entry
	for key := range cf.Entries {
		if cf.Entries[key].f == nil {
			// skip files that haven't been loaded, hash should already be defined
			continue
		}
		h, err := cf.Entries[key].f.Hash()
		if err != nil {
			return "", err
		}
		cf.Entries[key].Hash = h
	}
	// serialize cf into a byte string and hash it
	b, err := json.Marshal(cf)
	if err != nil {
		return "", err
	}
	hash, err := hasher.FromBytes(b)
	if err != nil {
		return "", err
	}
	// write the serialized cf using it's hash name to the backing storage
	cf.hash = hash
	wc, err := cf.backing.Write(hash)
	if err != nil {
		return "", err
	}
	defer wc.Close()
	_, err = wc.Write(b)
	if err != nil {
		return "", err
	}
	// return the hash
	return hash, nil
}

// Raw returns the marshaled version of the complex file
func (cf *ComplexFile) Raw() []byte {
	// TODO: store and reuse raw content
	b, _ := json.Marshal(cf)
	return b
}

// TODO:
// cf.Walk: apply walk fn to each entry
