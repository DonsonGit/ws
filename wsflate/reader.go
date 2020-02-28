package wsflate

import (
	"io"
)

type Decompressor interface {
	io.Reader
}

type ReadResetter interface {
	Reset(io.Reader)
}

// Reader contains logic of reading compressed data from a source.
// Reader may be reused for different sources after its Reset() method call.
type Reader struct {
	// Source is a compressed data source that should be used for further
	// decompression.
	Source io.Reader

	// Decompressor is a required callback function which must create and
	// configure Decompressor instance for given io.Reader.
	//
	// Note that given io.Reader is not the same as the Source field.
	//
	// If Decompressor doesn't implement ReadResetter interface this function
	// will be called on each Reset() call.
	Decompressor func(io.Reader) Decompressor

	d   Decompressor
	err error
	sr  suffixedReader
}

func (r *Reader) init() {
	if r.d == nil {
		r.sr.suffix = compressionReadTail
		r.sr.reset(r.Source)
		r.d = r.Decompressor(&r.sr)
	}
}

// Reset resets Reader to decompress data from src.
func (r *Reader) Reset(src io.Reader) {
	r.init()
	r.err = nil
	r.Source = src
	r.sr.reset(src)
	if rr, _ := r.d.(ReadResetter); rr != nil {
		rr.Reset(&r.sr)
	} else {
		r.d = r.Decompressor(&r.sr)
	}
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (n int, err error) {
	r.init()
	if r.err != nil {
		return 0, r.err
	}
	return r.d.Read(p)
}

// Close closes reader and a Decompressor instance used under the hood (if it
// implements io.Closer interface).
func (r *Reader) Close() error {
	r.init()
	if r.err != nil {
		return r.err
	}
	if c, _ := r.d.(io.Closer); c != nil {
		return c.Close()
	}
	return nil
}
