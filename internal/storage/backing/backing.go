package backing

import (
	"io"

	"github.com/sudo-bmitch/reproducible-proxy/internal/config"
)

type Backing interface {
	Read(name string) (io.ReadCloser, error)
	Rename(tgt, src string) error
	Write(name string) (io.WriteCloser, error)
	Delete(name string) error
}

var registered = map[string]func(config.Config) Backing{}

func Register(name string, s func(config.Config) Backing) {
	registered[name] = s
}

func Get(c config.Config) Backing {
	if fn, ok := registered[c.Storage.Backing]; ok {
		return fn(c)
	}
	return nil
}
