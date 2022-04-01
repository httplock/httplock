package backing

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/httplock/httplock/internal/config"
)

type memory struct {
	mu   sync.Mutex
	file map[string]*[]byte
}
type memoryFile struct {
	data *[]byte
}

// register as a backing
func init() {
	Register("memory", func(c config.Config) Backing {
		m := memory{
			mu:   sync.Mutex{},
			file: map[string]*[]byte{},
		}
		return &m
	})
}

func (m *memory) Read(name string) (io.ReadCloser, error) {
	if data, ok := m.file[name]; ok {
		b := bytes.NewBuffer(*data)
		return ioutil.NopCloser(b), nil
	}
	return nil, fmt.Errorf("file not found")
}

func (m *memory) Rename(tgt, src string) error {
	if d, ok := m.file[src]; ok {
		m.file[tgt] = d
		delete(m.file, src)
		return nil
	}
	return fmt.Errorf("file not found")
}

func (m *memory) Write(name string) (io.WriteCloser, error) {
	m.mu.Lock()
	data := []byte{}
	m.file[name] = &data
	f := memoryFile{
		data: &data,
	}
	m.mu.Unlock()
	return &f, nil
}

func (m *memory) Delete(name string) error {
	m.mu.Lock()
	delete(m.file, name)
	m.mu.Unlock()
	return nil
}

func (f *memoryFile) Write(p []byte) (int, error) {
	*f.data = append(*f.data, p...)
	return len(p), nil
}

func (f *memoryFile) Close() error {
	return nil
}
