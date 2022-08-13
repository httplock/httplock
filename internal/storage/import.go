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

func Import(s Storage, id string, r io.Reader) error {
	// make tmp tmpDir, defer cleanup
	tmpDir, err := os.MkdirTemp("", "httplock-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
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
		fn := filepath.Join(tmpDir, filepath.Clean("/"+th.Name))
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
	fh, err := os.Open(filepath.Join(tmpDir, filenameHTTPLock))
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
	fh, err = os.Open(filepath.Join(tmpDir, filenameIndexJSON))
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

	// recursively import the referenced blobs, and add the root
	dir, err := importDir(s, id, tmpDir)
	if err != nil {
		return err
	}
	newRoot := &Root{
		storage: s,
		dir:     dir,
	}
	hash, err := s.RootSave(newRoot)
	if err != nil {
		return err
	}
	if hash != id {
		return fmt.Errorf("hash mismatch, expected %s, computed %s", id, hash)
	}
	return nil
}

func importDir(s Storage, hash string, tmpDir string) (*Dir, error) {
	dir := Dir{
		hash: hash,
	}
	// read blob from tmpDir
	fh, err := os.Open(filepath.Join(tmpDir, filepath.Clean("/"+hash)))
	if err != nil {
		return nil, err
	}
	dirBytes, err := io.ReadAll(fh)
	fh.Close()
	if err != nil {
		return nil, err
	}
	// parse into json
	err = json.Unmarshal(dirBytes, &dir)
	if err != nil {
		return nil, err
	}
	// push to storage
	bw, err := s.BlobCreate()
	if err != nil {
		return nil, err
	}
	_, err = bw.Write(dirBytes)
	bw.Close()
	if err != nil {
		return nil, err
	}
	// verify hash
	bwHash, err := bw.Hash()
	if err != nil {
		return nil, err
	}
	if bwHash != hash {
		return nil, fmt.Errorf("hash mismatch, computed %s, expected %s", bwHash, hash)
	}
	// recursively load entries
	for name, entry := range dir.Entries {
		switch entry.Kind {
		case KindDir:
			eDir, err := importDir(s, entry.Hash, tmpDir)
			if err != nil {
				return nil, err
			}
			entry.dir = eDir
		case KindFile:
			eFile, err := importFile(s, entry.Hash, tmpDir)
			if err != nil {
				return nil, err
			}
			entry.file = eFile
		default:
			return nil, fmt.Errorf("unknown kind %d for entry %s", entry.Kind, name)
		}
	}
	return &dir, nil
}

func importFile(s Storage, hash string, tmpDir string) (*File, error) {
	file := File{
		hash: hash,
	}
	// read blob from tmpDir
	fh, err := os.Open(filepath.Join(tmpDir, filepath.Clean("/"+hash)))
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	if err != nil {
		return nil, err
	}
	// push to storage
	bw, err := s.BlobCreate()
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(bw, fh)
	bw.Close()
	if err != nil {
		return nil, err
	}
	// verify hash
	bwHash, err := bw.Hash()
	if err != nil {
		return nil, err
	}
	if bwHash != hash {
		return nil, fmt.Errorf("hash mismatch, computed %s, expected %s", bwHash, hash)
	}
	return &file, nil
}
