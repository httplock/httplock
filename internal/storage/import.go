package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func (s *Storage) Import(id string, r io.Reader) error {
	// make tmp dir, defer cleanup
	dir, err := os.MkdirTemp("", "httplock-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	// decompress
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	// extract tar to a temp dir
	tr := tar.NewReader(gr)
	for {
		th, err := tr.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		fn := filepath.Join(dir, filepath.Clean("/"+th.Name))
		switch th.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(fn, fs.FileMode(th.Mode))
			if err != nil {
				return err
			}
		case tar.TypeReg:
			fh, err := os.Create(fn)
			if err != nil {
				return err
			}
			n, err := io.Copy(fh, tr)
			fh.Close()
			if err != nil {
				return err
			}
			if n != th.Size {
				return err
			}
		}
	}
	// verify httplock version
	fh, err := os.Open(filepath.Join(dir, filenameHTTPLock))
	if err != nil {
		return err
	}
	fhlBytes, err := io.ReadAll(fh)
	fh.Close()
	if err != nil {
		return err
	}
	fhl := fileHTTPLock{}
	err = json.Unmarshal(fhlBytes, &fhl)
	if err != nil {
		return err
	}
	if fhl.Version != "1.0" {
		return err
	}

	// load index.json
	fh, err = os.Open(filepath.Join(dir, filenameIndexJSON))
	if err != nil {
		return err
	}
	indBytes, err := io.ReadAll(fh)
	fh.Close()
	if err != nil {
		return err
	}
	ind := Index{}
	err = json.Unmarshal(indBytes, &ind)
	if err != nil {
		return err
	}
	// verify selected root exists
	if _, ok := ind.Roots[id]; !ok {
		return err
	}
	// recursively load each directory, complex file, and file
	newRoot, err := s.importDir(dir, id)
	if err != nil {
		return err
	}
	// add to list of known roots
	s.roots[id] = newRoot
	// add entry to the index
	s.usedRoot(id)
	err = s.WriteIndex()
	if err != nil {
		return err
	}
	return nil
}

func (s *Storage) importDir(baseDir, hash string) (*Dir, error) {
	dir, err := DirNew(s.Backing)
	if err != nil {
		return nil, err
	}
	fh, err := os.Open(filepath.Join(baseDir, filepath.Clean("/"+hash)))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	jd := json.NewDecoder(fh)
	err = jd.Decode(&dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range dir.Entries {
		switch entry.Kind {
		case entryDir:
			dirNew, err := s.importDir(baseDir, entry.Hash)
			if err != nil {
				return nil, err
			}
			entry.dir = dirNew
		case entryComplex:
			cNew, err := s.importComplex(baseDir, entry.Hash)
			if err != nil {
				return nil, err
			}
			entry.complex = cNew

		case entryFile:
			fNew, err := s.importFile(baseDir, entry.Hash)
			if err != nil {
				return nil, err
			}
			entry.file = fNew

		default:
			return nil, fmt.Errorf("unhandled entry, hash %s, kind %v", hash, entry.Kind)
		}
	}
	// verify hash
	hashNew, err := dir.Hash()
	if err != nil {
		return nil, err
	}
	if hashNew != hash {
		return nil, fmt.Errorf("hash mismatch, expected %s, read %s", hash, hashNew)
	}
	return dir, nil
}

func (s *Storage) importComplex(baseDir, hash string) (*ComplexFile, error) {
	cFile, err := ComplexNew(s.Backing)
	if err != nil {
		return nil, err
	}
	fh, err := os.Open(filepath.Join(baseDir, filepath.Clean("/"+hash)))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	jd := json.NewDecoder(fh)
	err = jd.Decode(&cFile)
	if err != nil {
		return nil, err
	}
	for name, ce := range cFile.Entries {
		cef, err := s.importFile(baseDir, ce.Hash)
		if err != nil {
			return nil, err
		}
		cFile.Entries[name].f = cef
	}
	// verify hash
	hashNew, err := cFile.Hash()
	if err != nil {
		return nil, err
	}
	if hashNew != hash {
		return nil, fmt.Errorf("hash mismatch, expected %s, read %s", hash, hashNew)
	}
	return cFile, nil

}

func (s *Storage) importFile(baseDir, hash string) (*File, error) {
	file, err := FileNew(s.Backing)
	if err != nil {
		return nil, err
	}
	fh, err := os.Open(filepath.Join(baseDir, filepath.Clean("/"+hash)))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	fw, err := file.Write()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(fw, fh)
	fw.Close()
	if err != nil {
		return nil, err
	}
	// verify hash
	hashNew, err := file.Hash()
	if err != nil {
		return nil, err
	}
	if hashNew != hash {
		return nil, fmt.Errorf("hash mismatch, expected %s, read %s", hash, hashNew)
	}
	return file, nil
}
