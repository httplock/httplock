package proxy

import "io"

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
