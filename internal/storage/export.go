package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
)

func (s *Storage) Export(root *Dir, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// add httplock file with version
	fhl := fileHTTPLock{Version: "1.0"}
	fhlBytes, err := json.Marshal(fhl)
	if err != nil {
		return err
	}
	err = tarAdd(tw, filenameHTTPLock, fhlBytes)
	if err != nil {
		return err
	}

	// add index.json with root array entry
	rHash, err := root.Hash()
	if err != nil {
		return err
	}
	ind := Index{
		Roots: map[string]*IndexRoot{
			rHash: {},
		},
	}
	ij, err := json.Marshal(ind)
	if err != nil {
		return err
	}
	err = tarAdd(tw, filenameIndexJSON, ij)
	if err != nil {
		return err
	}

	// recursively add child entries from the root
	s.Walk([]string{}, root, WalkFuncs{
		Dir: func(path []string, dir *Dir) error {
			hash, err := dir.Hash()
			if err != nil {
				return err
			}
			raw := dir.Raw()
			tarAdd(tw, hash, raw)
			return nil
		},
		Complex: func(path []string, cf *ComplexFile) error {
			hash, err := cf.Hash()
			if err != nil {
				return err
			}
			raw := cf.Raw()
			tarAdd(tw, hash, raw)
			return nil
		},
		File: func(path []string, file *File) error {
			hash, err := file.Hash()
			if err != nil {
				return err
			}
			size := file.Size()
			rc, err := file.Read()
			if err != nil {
				return err
			}
			defer rc.Close()
			tarAddReader(tw, hash, size, rc)
			return nil
		},
	})
	return nil
}

func tarAdd(tw *tar.Writer, name string, data []byte) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 0755,
	})
	if err != nil {
		return err
	}
	_, err = tw.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func tarAddReader(tw *tar.Writer, name string, size int64, rdr io.Reader) error {
	err := tw.WriteHeader(&tar.Header{
		Name: name,
		Size: size,
		Mode: 0755,
	})
	if err != nil {
		return err
	}
	_, err = io.Copy(tw, rdr)
	if err != nil {
		return err
	}
	return nil
}
