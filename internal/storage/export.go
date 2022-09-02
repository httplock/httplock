package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
)

// Export outputs a tgz to a writer for a specific root
func Export(s Storage, id string, w io.Writer) error {
	root, err := s.RootOpen(id)
	if err != nil {
		return err
	}

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
	ind := Index{
		Roots: map[string]*IndexRoot{
			root.hash: {},
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

	// add index.md
	report, err := root.report()
	if err != nil {
		return err
	}
	err = tarAdd(tw, filenameIndexMD, []byte("# Index\n\n"+report))
	if err != nil {
		return err
	}

	err = root.Walk(WalkFns{
		fnDir: func(d *Dir) error {
			err := root.hashDir(d)
			if err != nil {
				return err
			}
			if d.hash == "" {
				return fmt.Errorf("hash missing for directory")
			}
			br, err := s.BlobOpen(d.hash)
			if err != nil {
				return err
			}
			defer br.Close()
			return tarAddReader(tw, d.hash, br.Size(), br)
		},
		fnFile: func(f *File) error {
			err := root.hashFile(f)
			if err != nil {
				return err
			}
			if f.hash == "" {
				return fmt.Errorf("hash missing for file")
			}
			br, err := s.BlobOpen(f.hash)
			if err != nil {
				return err
			}
			defer br.Close()
			return tarAddReader(tw, f.hash, br.Size(), br)
		},
	})
	if err != nil {
		return err
	}
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
