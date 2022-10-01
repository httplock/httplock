package storage

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/httplock/httplock/internal/config"
)

func TestRoot(t *testing.T) {
	filePathCommon := []string{"dir2", "file-common"}
	filePath1A := []string{"dir1", "fileA"}
	filePath1B := []string{"dir1", "fileB"}
	filePath3A := []string{"dir3", "fileA"}
	filePath3B := []string{"dir3", "fileB"}
	fileCommon := []byte("test common file")
	file1A := []byte("dir1 - fileA")
	file1Ba := []byte("dir1 - fileBa")
	file1Bb := []byte("dir1 - fileBb")
	file3A := []byte("dir3 - fileA")
	file3B := []byte("dir3 - fileB")
	conf := `{"storage": {"kind": "memory"}}`
	c := config.Config{}
	err := config.LoadReader(strings.NewReader(conf), &c)
	if err != nil {
		t.Errorf("failed to read config: %v", err)
		return
	}
	s, err := Get(c)
	if err != nil {
		t.Errorf("failed to load storage: %v", err)
		return
	}
	// setup first root
	_, r1, err := s.RootCreate()
	if err != nil {
		t.Errorf("failed to create root 1: %v", err)
		return
	}
	// add common file
	bw, err := r1.Write(filePathCommon)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(fileCommon)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}
	// save root to get hash, open root2 from common, and root common
	hashCommon, err := s.RootSave(r1)
	if err != nil {
		t.Errorf("failed to save root: %v", err)
		return
	}
	_, r2, err := s.RootCreateFrom(hashCommon)
	if err != nil {
		t.Errorf("failed to create root 2: %v", err)
		return
	}
	rCommon, err := s.RootOpen(hashCommon)
	if err != nil {
		t.Errorf("failed to create root common: %v", err)
		return
	}
	// add to r1 and r2
	bw, err = r1.Write(filePath1A)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(file1A)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}
	bw, err = r1.Write(filePath1B)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(file1Ba)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}
	bw, err = r1.Write(filePath3B)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(file3B)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}

	bw, err = r2.Write(filePath1B)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(file1Bb)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}
	bw, err = r2.Write(filePath3A)
	if err != nil {
		t.Errorf("failed to create writer: %v", err)
		return
	}
	_, err = bw.Write(file3A)
	if err != nil {
		t.Errorf("failed to write blob: %v", err)
		return
	}
	err = bw.Close()
	if err != nil {
		t.Errorf("failed to close blob: %v", err)
		return
	}

	// verify rCommon is read-only
	bw, err = rCommon.Write(filePath1A)
	if err == nil {
		t.Errorf("rCommon is not read-only")
		bw.Close()
	}

	// read from roots
	br, err := rCommon.Read(filePathCommon)
	if err != nil {
		t.Errorf("Failed to read common file: %v", err)
		return
	}
	b, err := io.ReadAll(br)
	if err != nil {
		t.Errorf("Failed to ReadAll: %v", err)
		return
	}
	if !bytes.Equal(b, fileCommon) {
		t.Errorf("file content mismatch, expected %s, received %s", string(fileCommon), string(b))
	}
	err = br.Close()
	if err != nil {
		t.Errorf("failed to close blob reader: %v", err)
	}

	br, err = rCommon.Read(filePath1A)
	if err == nil {
		t.Errorf("Common read found file 1a")
		br.Close()
	}

	br, err = r1.Read(filePathCommon)
	if err != nil {
		t.Errorf("Failed to read common file: %v", err)
		return
	}
	b, err = io.ReadAll(br)
	if err != nil {
		t.Errorf("Failed to ReadAll: %v", err)
		return
	}
	if !bytes.Equal(b, fileCommon) {
		t.Errorf("file content mismatch, expected %s, received %s", string(fileCommon), string(b))
	}
	err = br.Close()
	if err != nil {
		t.Errorf("failed to close blob reader: %v", err)
	}

	br, err = r2.Read(filePathCommon)
	if err != nil {
		t.Errorf("Failed to read common file: %v", err)
		return
	}
	b, err = io.ReadAll(br)
	if err != nil {
		t.Errorf("Failed to ReadAll: %v", err)
		return
	}
	if !bytes.Equal(b, fileCommon) {
		t.Errorf("file content mismatch, expected %s, received %s", string(fileCommon), string(b))
	}
	err = br.Close()
	if err != nil {
		t.Errorf("failed to close blob reader: %v", err)
	}

	br, err = r1.Read(filePath1A)
	if err != nil {
		t.Errorf("Failed to read 1a file: %v", err)
		return
	}
	b, err = io.ReadAll(br)
	if err != nil {
		t.Errorf("Failed to ReadAll: %v", err)
		return
	}
	if !bytes.Equal(b, file1A) {
		t.Errorf("file content mismatch, expected %s, received %s", string(file1A), string(b))
	}
	err = br.Close()
	if err != nil {
		t.Errorf("failed to close blob reader: %v", err)
	}

	br, err = r2.Read(filePath3A)
	if err != nil {
		t.Errorf("Failed to read 3a file: %v", err)
		return
	}
	b, err = io.ReadAll(br)
	if err != nil {
		t.Errorf("Failed to ReadAll: %v", err)
		return
	}
	if !bytes.Equal(b, file3A) {
		t.Errorf("file content mismatch, expected %s, received %s", string(fileCommon), string(b))
	}
	err = br.Close()
	if err != nil {
		t.Errorf("failed to close blob reader: %v", err)
	}

	// diff rCommon, r1, and r2
	dr, err := DiffRoots(rCommon, rCommon)
	if err != nil {
		t.Errorf("failed to generate diff rCommon with self: %v", err)
		return
	}
	if len(dr.Entries) != 0 {
		t.Errorf("diff entries expected 0, received %d", len(dr.Entries))
		return
	}
	dr, err = DiffRoots(r1, r2)
	if err != nil {
		t.Errorf("failed to generate diff r1 - r2: %v", err)
		return
	}
	if len(dr.Entries) != 4 {
		t.Errorf("diff entries expected 2, received %d", len(dr.Entries))
		return
	}
	// marshal report and log
	drj, err := json.MarshalIndent(dr, "", "  ")
	if err != nil {
		t.Errorf("failed to marshal diff report: %v", err)
		return
	}
	t.Logf("Report:\n%s", string(drj))

}
