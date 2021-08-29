package backing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/httplock/httplock/internal/config"
)

func TestFilesystem(t *testing.T) {
	var testCases = []struct {
		name  string
		input []byte
		hash  string
	}{
		{
			name:  "empty",
			input: []byte{},
			hash:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "hello",
			input: []byte("Hello world"),
			hash:  "sha256:64ec88ca00b268e5ba1a35678a1b5316d212f4f366b2477232534a8aeca37f3c",
		},
	}

	tempDir, err := ioutil.TempDir("", "gotest-")
	if err != nil {
		t.Errorf("TempDir creation error: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	c := config.Config{}
	c.Storage.Backing = "filesystem"
	c.Storage.Filesystem.Directory = tempDir
	fBacking := Get(c)

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("write-%s", tt.name), func(t *testing.T) {
			fw, err := fBacking.Write(tt.hash)
			if err != nil {
				t.Errorf("File backing write error: %v", err)
			}
			n, err := fw.Write(tt.input)
			if err != nil {
				t.Errorf("File write error: %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write length: got %d, expected %d", n, len(tt.input))
			}
			err = fw.Close()
			if err != nil {
				t.Errorf("File close error: %v", err)
			}
		})
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("read-%s", tt.name), func(t *testing.T) {
			fr, err := fBacking.Read(tt.hash)
			if err != nil {
				t.Errorf("File backing read error: %v", err)
			}
			mrb, err := ioutil.ReadAll(fr)
			if err != nil {
				t.Errorf("File read error: %v", err)
			}
			if bytes.Compare(mrb, tt.input) != 0 {
				t.Errorf("File read: got %s, expected %s", mrb, tt.input)
			}
		})
	}

	// TODO: read missing file

	// TODO: delete files

}
