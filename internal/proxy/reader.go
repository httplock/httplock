package proxy

import (
	"io"
)

type teeReadCloser struct {
	io.Reader
	r  io.ReadCloser
	w  io.WriteCloser
	cb func() error
}

// wrap a tee reader to handle Closer variants
func newTeeRC(r io.ReadCloser, w io.WriteCloser, cb func() error) io.ReadCloser {
	tr := io.TeeReader(r, w)
	trc := teeReadCloser{
		Reader: tr,
		r:      r,
		w:      w,
		cb:     cb,
	}
	return &trc
}

// pass through close requests
func (trc *teeReadCloser) Close() error {
	errs := []error{}
	errs = append(errs, trc.r.Close())
	errs = append(errs, trc.w.Close())
	if trc.cb != nil {
		errs = append(errs, trc.cb())
	}
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

type newRC func() (io.ReadCloser, error)
type hashReadCloser struct {
	io.ReadCloser
	h     string
	newRC newRC
}

func newHashRC(r io.ReadCloser, h string, nrc newRC) (*hashReadCloser, error) {
	return &hashReadCloser{
		ReadCloser: r,
		h:          h,
		newRC:      nrc,
	}, nil
}

func (hrc *hashReadCloser) Reset() (io.ReadCloser, error) {
	rc, err := hrc.newRC()
	if err != nil {
		return hrc, err
	}
	hrc.ReadCloser = rc
	return hrc, nil
}
