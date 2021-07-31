package backing

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
)

func TestMemory(t *testing.T) {
	var testMem = []struct {
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

	m := Get("memory")

	for _, tt := range testMem {
		t.Run(fmt.Sprintf("write-%s", tt.name), func(t *testing.T) {
			mw, err := m.Write(tt.hash)
			if err != nil {
				t.Errorf("Memory write error: %v", err)
			}
			n, err := mw.Write(tt.input)
			if err != nil {
				t.Errorf("File write error: %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write length: got %d, expected %d", n, len(tt.input))
			}
			err = mw.Close()
			if err != nil {
				t.Errorf("File close error: %v", err)
			}
		})
	}

	for _, tt := range testMem {
		t.Run(fmt.Sprintf("read-%s", tt.name), func(t *testing.T) {
			mr, err := m.Read(tt.hash)
			if err != nil {
				t.Errorf("Memory read error: %v", err)
			}
			mrb, err := ioutil.ReadAll(mr)
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
