package backing

import "io"

type Backing interface {
	Read(name string) (io.ReadCloser, error)
	Rename(tgt, src string) error
	Write(name string) (io.WriteCloser, error)
	Delete(name string) error
}

var registered = map[string]func() Backing{}

func Register(name string, s func() Backing) {
	registered[name] = s
}

func Get(name string) Backing {
	if fn, ok := registered[name]; ok {
		return fn()
	}
	return nil
}
