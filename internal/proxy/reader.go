package proxy

import (
	"bytes"
	"io"

	"github.com/httplock/httplock/hasher"
)

type teeReadCloser struct {
	io.Reader
	r io.ReadCloser
	w io.WriteCloser
}

// wrap a tee reader to handle Closer variants
func newTeeRC(r io.ReadCloser, w io.WriteCloser) io.ReadCloser {
	tr := io.TeeReader(r, w)
	trc := teeReadCloser{
		Reader: tr,
		r:      r,
		w:      w,
	}
	return &trc
}

// pass through close requests
func (trc *teeReadCloser) Close() error {
	errR := trc.r.Close()
	errW := trc.w.Close()
	if errR != nil {
		return errR
	}
	if errW != nil {
		return errW
	}
	return nil
}

type hashReadCloser struct {
	io.Reader
	h   string
	buf []byte
}

func newHashRC(r io.ReadCloser) (*hashReadCloser, error) {
	// if reader is nil, hash an empty string, and return a hashReadCloser with an empty buffer reader
	if r == nil {
		buf := []byte{}
		br := bytes.NewReader(buf)
		h, err := hasher.FromBytes(buf)
		if err != nil {
			return nil, err
		}
		hrc := hashReadCloser{
			Reader: br,
			h:      h,
			buf:    buf,
		}
		return &hrc, nil
	}
	// TODO: always write to a tmp file in the storage backing, hash the content, rename to hash value, and using backing reader of hash

	// otherwise stream the read through a hasher into a buffer, return a reader on the buffer
	hr := hasher.NewReader(r)

	buf, err := io.ReadAll(hr)
	if err != nil {
		return nil, err
	}
	r.Close()
	br := bytes.NewReader(buf)
	hrc := hashReadCloser{
		Reader: br,
		h:      hr.String(),
	}
	return &hrc, nil
}

func (hrc *hashReadCloser) GetReader() (io.ReadCloser, error) {
	hrc.Reader = bytes.NewReader(hrc.buf)
	return hrc, nil
}

func (hrc *hashReadCloser) Close() error {
	// TODO: if saving large files to the backend, close and delete the temporary file
	return nil
}
