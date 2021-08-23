package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sudo-bmitch/reproducible-proxy/config"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

const (
	filenameIndexJSON = "index.json"
	filenameIndexMD   = "index.md"
)

type Storage struct {
	Backing backing.Backing
	roots   map[string]*Dir
	index   Index
}

type Index struct {
	Roots map[string]*IndexRoot `json:"roots"`
}

type IndexRoot struct {
	Used time.Time `json:"used,omitempty"`
}

func New(c config.Config) (*Storage, error) {
	b := backing.Get(c)
	if b == nil {
		return nil, fmt.Errorf("Backing not found: %s", c.Storage.Backing)
	}
	s := Storage{
		Backing: b,
		roots:   map[string]*Dir{},
		index: Index{
			Roots: map[string]*IndexRoot{},
		},
	}
	// load the index.json if it exists
	ir, err := b.Read(filenameIndexJSON)
	if err == nil {
		defer ir.Close()
		json.NewDecoder(ir).Decode(&s.index)
	}
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
		s.usedRoot(name)
		return d, nil
	}
	d, err := DirLoad(s.Backing, name)
	if err == nil {
		s.usedRoot(name)
		return d, nil
	}
	return nil, fmt.Errorf("Root not found: %s", name)
}

func (s *Storage) ListRoots() ([]string, error) {
	keys := make([]string, 0, len(s.roots))
	for k := range s.roots {
		keys = append(keys, k)
	}
	// include entries from index that aren't in roots map
	for h := range s.index.Roots {
		if _, ok := s.roots[h]; !ok {
			keys = append(keys, h)
		}
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
	// add entry to the index
	s.usedRoot(h)
	err = s.WriteIndex()
	if err != nil {
		return h, err
	}
	return h, nil
}

func (s *Storage) usedRoot(name string) {
	// don't write uuid's to index
	if strings.HasPrefix(name, "uuid:") {
		return
	}
	if _, ok := s.index.Roots[name]; !ok {
		s.index.Roots[name] = &IndexRoot{}
	}
	s.index.Roots[name].Used = time.Now()
}

func (s *Storage) WriteIndex() error {
	// fmt.Printf("Writing index\n")
	// s.Backing.Delete(filenameIndexJSON)
	ij, err := s.Backing.Write(filenameIndexJSON)
	if err != nil {
		return err
	}
	defer ij.Close()
	err = json.NewEncoder(ij).Encode(s.index)
	if err != nil {
		return err
	}

	// s.Backing.Delete(filenameIndexMD)
	im, err := s.Backing.Write(filenameIndexMD)
	if err != nil {
		return err
	}
	defer im.Close()

	// write header
	_, err = fmt.Fprintf(im, "# Index\n")
	if err != nil {
		return err
	}
	// walk to get the URL's, show each request/response/body as links to individual files
	lastURL := ""
	handleDir := func(path []string, dir *Dir) error {
		// fmt.Printf("walking dir %s\n", strings.Join(path, ""))
		return nil
	}
	handleComplex := func(path []string, cf *ComplexFile) error {
		if len(path) < 3 {
			return fmt.Errorf("invalid path: %v", path)
		}
		curURL := fmt.Sprintf("`%s://%s/%s`", path[0], path[1], strings.Join(path[2:len(path)-1], ""))
		name := path[len(path)-1]
		if curURL != lastURL {
			_, err := fmt.Fprintf(im, "- %s\n", curURL)
			if err != nil {
				return err
			}
		}
		cfReq, ok := cf.Entries["meta-req"]
		if !ok {
			return fmt.Errorf("missing meta-req in %s", name)
		}
		cfResp, ok := cf.Entries["meta-resp"]
		if !ok {
			return fmt.Errorf("missing meta-resp in %s", name)
		}
		cfBody, ok := cf.Entries["body"]
		if !ok {
			return fmt.Errorf("missing body in %s", name)
		}
		_, err := fmt.Fprintf(im, "  - [Req](./%s) /\n    [Resp](./%s) /\n    [Body](./%s)\n", cfReq.Hash, cfResp.Hash, cfBody.Hash)
		if err != nil {
			return err
		}
		return nil
	}

	// loop over each root to output
	for rootName := range s.index.Roots {
		root, ok := s.roots[rootName]
		if !ok || root == nil {
			root, err = s.GetRoot(rootName)
			if err != nil {
				return err
			}
		}
		if root == nil {
			return fmt.Errorf("root missing %s", rootName)
		}

		_, err = fmt.Fprintf(im, "\n## [%s](./%s)\n\n", rootName, rootName)
		if err != nil {
			return err
		}

		// reset lastURL so handleComplex always outputs the first url under each root
		lastURL = ""

		err = s.walk([]string{}, root, walkFuncs{dir: handleDir, complex: handleComplex})
		if err != nil {
			return err
		}
	}
	return nil
}
