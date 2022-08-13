package storage

import (
	"fmt"
	"io"

	"github.com/httplock/httplock/hasher"
)

// BlobWriter is used when writing blobs
type BlobWriter interface {
	io.WriteCloser
	// Hash is available after the writer is closed
	Hash() (string, error)
}

type blobWrite struct {
	orig    io.Writer
	closed  bool
	closeFn closeFn
	hash    *hasher.Writer
}

type closeFn func(hash string) error

func newBlobWriter(w io.Writer, cfn closeFn) *blobWrite {
	return &blobWrite{
		orig:    w,
		hash:    hasher.NewWriter(w),
		closeFn: cfn,
	}
}

func (bw *blobWrite) Close() error {
	if bw.closed {
		return nil
	}
	errs := []error{}
	bw.closed = true
	if bwc, ok := bw.orig.(io.Closer); ok {
		err := bwc.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if bw.closeFn != nil {
		err := bw.closeFn(bw.hash.String())
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		errJoin := errs[0]
		for _, add := range errs[1:] {
			// not exactly what I want, but this should be rare
			errJoin = fmt.Errorf("%v, %w", errJoin, add)
		}
		return errJoin
	}
	return nil
}

func (bw *blobWrite) Hash() (string, error) {
	if !bw.closed {
		return "", fmt.Errorf("hash unavailable, writer is not closed")
	}
	return bw.hash.String(), nil
}

func (bw *blobWrite) Write(p []byte) (n int, err error) {
	return bw.hash.Write(p)
}
