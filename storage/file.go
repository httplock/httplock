package storage

import (
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/sudo-bmitch/reproducible-proxy/hasher"
	"github.com/sudo-bmitch/reproducible-proxy/storage/backing"
)

type File struct {
	hash    string
	temp    string
	backing backing.Backing
}

type fileHash struct {
	f  *File
	bw io.WriteCloser
	hw *hasher.Writer
}

func FileNew(backing backing.Backing) (*File, error) {
	f := File{
		backing: backing,
	}
	return &f, nil
}

// FileLoad: load a file based on hash
func FileLoad(backing backing.Backing, hash string) (*File, error) {
	f := File{
		hash:    hash,
		backing: backing,
	}
	return &f, nil
}

func (f *File) Hash() (string, error) {
	return f.hash, nil
}

func (f *File) Delete() error {
	if f.temp != "" {
		return f.backing.Delete(f.temp)
	}
	if f.hash != "" {
		return f.backing.Delete(f.hash)
	}
	return fmt.Errorf("File has no backing file")
}

func (f *File) Read() (io.ReadCloser, error) {
	if f.hash != "" {
		return f.backing.Read(f.hash)
	}
	return nil, fmt.Errorf("File has no backing file")
}

func (f *File) Write() (io.WriteCloser, error) {
	// write to a temp file
	u := fmt.Sprintf("temp-%s", uuid.New().String())
	f.temp = u
	bw, err := f.backing.Write(f.temp)
	if err != nil {
		return nil, err
	}
	fh := fileHash{
		f:  f,
		bw: bw,
		hw: hasher.NewWriter(bw),
	}
	return &fh, nil
}

func (fh *fileHash) Write(p []byte) (int, error) {
	return fh.hw.Write(p)
}

func (fh *fileHash) Close() error {
	// store hash
	fh.f.hash = fh.hw.String()
	// rename temp file to hash file
	err := fh.f.backing.Rename(fh.f.hash, fh.f.temp)
	if err != nil {
		return err
	}
	fh.f.temp = ""
	return fh.bw.Close()
}

// TODO: is custom MarshalJSON needed?
