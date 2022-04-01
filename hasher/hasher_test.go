package hasher

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestHashReader(t *testing.T) {
	var testHash = []struct {
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

	for _, tt := range testHash {
		t.Run(tt.name, func(t *testing.T) {
			buf := bytes.NewBuffer(tt.input)
			h := NewReader(buf)
			result, err := ioutil.ReadAll(h)
			if err != nil {
				t.Errorf("Error reading from hasher: %v", err)
			}
			if !bytes.Equal(result, tt.input) {
				t.Errorf("Read got bytes %s, expected %s", result, tt.input)
			}
			if h.String() != tt.hash {
				t.Errorf("Read got hash %s, expected %s", h.String(), tt.hash)
			}
			hstr, err := FromBytes(tt.input)
			if err != nil {
				t.Errorf("Error hashing bytes: %v", err)
			} else if hstr != tt.hash {
				t.Errorf("FromBytes got hash %s, expected %s", hstr, tt.hash)
			}
		})
	}
}

// TODO: test Hasher Writer
