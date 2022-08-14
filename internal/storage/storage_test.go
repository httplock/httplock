package storage

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/httplock/httplock/hasher"
	"github.com/httplock/httplock/internal/config"
)

func TestStorage(t *testing.T) {
	sampleBlob := []byte("sample blob")
	sampleHash, err := hasher.FromBytes(sampleBlob)
	if err != nil {
		t.Errorf("failed to get sample blob hash: %v", err)
		return
	}
	tests := []struct {
		name string
		conf string
	}{
		{
			name: "memory",
			conf: `{"storage": {"kind": "memory"}}`,
		},
		{
			name: "filesystem",
			conf: fmt.Sprintf(`{"storage": {"kind": "filesystem", "directory": "%s"}}`, t.TempDir()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.Config{}
			err := config.LoadReader(strings.NewReader(tt.conf), &c)
			if err != nil {
				t.Errorf("failed to read config: %v", err)
				return
			}
			s, err := Get(c)
			if err != nil {
				t.Errorf("failed to load storage: %v", err)
				return
			}
			// verify opening a blob before it's been written fails
			_, err = s.BlobOpen(sampleHash)
			if err == nil {
				t.Errorf("opening a new blob unexpectedly succeeded")
			}
			// create, read, and compare read blob
			bw, err := s.BlobCreate()
			if err != nil {
				t.Errorf("failed to create blob: %v", err)
				return
			}
			_, err = bw.Write(sampleBlob)
			if err != nil {
				t.Errorf("failed to write to blob: %v", err)
			}
			_, err = bw.Hash()
			if err == nil {
				t.Errorf("hash before blob close unexpectedly succeeded")
			}
			err = bw.Close()
			if err != nil {
				t.Errorf("failed to close to blob: %v", err)
			}
			hash, err := bw.Hash()
			if err != nil {
				t.Errorf("blob hash failed: %v", err)
			}
			if hash != sampleHash {
				t.Errorf("blob hash mismatch, expected %s, computed %s", sampleHash, hash)
			}
			br, err := s.BlobOpen(sampleHash)
			if err != nil {
				t.Errorf("failed to open blob: %v", err)
			}
			blob, err := io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read blob: %v", err)
			}
			if !bytes.Equal(sampleBlob, blob) {
				t.Errorf("blob mismatch: expected %s, received %s", sampleBlob, blob)
			}
			// create a new root and test read/write
			id, root, err := s.RootCreate()
			if err != nil {
				t.Errorf("failed to create new root: %v", err)
			}
			_, err = root.Read([]string{"missing"})
			if err == nil {
				t.Error("read on missing root file unexpectedly succeeded")
			}
			bw, err = root.Write([]string{"path", "to", "file"})
			if err != nil {
				t.Errorf("create blob writer failed: %v", err)
			}
			_, err = bw.Write(sampleBlob)
			if err != nil {
				t.Errorf("write failed: %v", err)
			}
			_, err = root.Read([]string{"path", "to", "file"})
			if err == nil {
				t.Error("read on blob still being written succeeded")
			}
			bw.Close()
			br, err = root.Read([]string{"path", "to", "file"})
			if err != nil {
				t.Errorf("failed to open blob: %v", err)
			}
			blob, err = io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read blob: %v", err)
			}
			if !bytes.Equal(sampleBlob, blob) {
				t.Errorf("blob mismatch: expected %s, received %s", sampleBlob, blob)
			}
			rootHash, err := s.RootSave(root)
			if err != nil {
				t.Errorf("failed to save root: %v", err)
			}
			// open root by uuid
			rootByID, err := s.RootOpen(id)
			if err != nil {
				t.Errorf("failed to open root by uuid: %v", err)
			}
			br, err = rootByID.Read([]string{"path", "to", "file"})
			if err != nil {
				t.Errorf("failed to open blob: %v", err)
			}
			blob, err = io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read blob: %v", err)
			}
			if !bytes.Equal(sampleBlob, blob) {
				t.Errorf("blob mismatch: expected %s, received %s", sampleBlob, blob)
			}
			// open root by hash
			rootByHash, err := s.RootOpen(rootHash)
			if err != nil {
				t.Errorf("failed to open root by hash: %v", err)
			}
			br, err = rootByHash.Read([]string{"path", "to", "file"})
			if err != nil {
				t.Errorf("failed to open blob: %v", err)
			}
			blob, err = io.ReadAll(br)
			if err != nil {
				t.Errorf("failed to read blob: %v", err)
			}
			if !bytes.Equal(sampleBlob, blob) {
				t.Errorf("blob mismatch: expected %s, received %s", sampleBlob, blob)
			}

		})
	}
}
