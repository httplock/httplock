package storage

import "io"

// BlobReader is used to read blobs
type BlobReader interface {
	io.ReadSeekCloser
	Size() int64
}

type blobRead struct {
	orig io.ReadSeeker
	size int64
}

func newBlobReader(rdr io.ReadSeeker, size int64) (*blobRead, error) {
	return &blobRead{
		orig: rdr,
		size: size,
	}, nil
}

// Close passed through the close request if available, noop otherwise
func (br *blobRead) Close() error {
	c, ok := br.orig.(io.Closer)
	if ok {
		return c.Close()
	}
	return nil
}

// Read passes through the read request
func (br *blobRead) Read(p []byte) (n int, err error) {
	return br.orig.Read(p)
}

// Seek passes through the seek request
func (br *blobRead) Seek(offset int64, whence int) (int64, error) {
	return br.orig.Seek(offset, whence)
}

func (br *blobRead) Size() int64 {
	return br.size
}
