package backing

import (
	"io"
	"os"
	"path/filepath"

	"github.com/sudo-bmitch/reproducible-proxy/config"
)

type filesystem struct {
	dir string
}

// register as a backing
func init() {
	Register("filesystem", func(c config.Config) Backing {
		dir := c.Storage.Filesystem.Directory
		if dir == "" || dir == "/" {
			// refuse to start if directory is empty or root filesystem
			return nil
		}
		f := filesystem{
			dir: dir,
		}
		return &f
	})
}

func (f *filesystem) Read(name string) (io.ReadCloser, error) {
	name = filepath.Join(f.dir, filepath.Clean(name))
	return os.Open(name)
}

func (f *filesystem) Rename(tgt, src string) error {
	tgt = filepath.Join(f.dir, filepath.Clean(tgt))
	src = filepath.Join(f.dir, filepath.Clean(src))
	return os.Rename(src, tgt)
}

func (f *filesystem) Write(name string) (io.WriteCloser, error) {
	name = filepath.Join(f.dir, filepath.Clean(name))
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
}

func (f *filesystem) Delete(name string) error {
	name = filepath.Join(f.dir, filepath.Clean(name))
	return os.Remove(name)
}
