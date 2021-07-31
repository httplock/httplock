package hasher

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
)

const algorithm = "sha256"

type Reader struct {
	h hash.Hash
	r io.Reader
}

type Writer struct {
	h hash.Hash
	w io.Writer
}

func FromBytes(b []byte) (string, error) {
	s := sha256.New()
	_, err := s.Write(b)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%x", algorithm, s.Sum(nil)), nil
}

func NewReader(r io.Reader) *Reader {
	h := Reader{
		h: sha256.New(),
		r: r,
	}
	return &h
}

func (hr *Reader) Read(p []byte) (int, error) {
	i, err := hr.r.Read(p)
	if i > 0 {
		_, err := hr.h.Write(p[:i])
		if err != nil {
			return i, err
		}
	}
	return i, err
}

func (hr *Reader) String() string {
	return fmt.Sprintf("%s:%x", algorithm, hr.h.Sum(nil))
}

func NewWriter(w io.Writer) *Writer {
	h := Writer{
		h: sha256.New(),
		w: w,
	}
	return &h
}

func (hw *Writer) Write(p []byte) (int, error) {
	i, err := hw.w.Write(p)
	if i > 0 {
		_, err := hw.h.Write(p[:i])
		if err != nil {
			return i, err
		}
	}
	return i, err
}

func (hw *Writer) String() string {
	return fmt.Sprintf("%s:%x", algorithm, hw.h.Sum(nil))
}
