package wsflate

import (
	"fmt"
	"io"
)

var (
	compressionTail = [4]byte{
		0, 0, 0xff, 0xff,
	}
	compressionReadTail = [9]byte{
		0, 0, 0xff, 0xff,
		1,
		0, 0, 0xff, 0xff,
	}
)

type Compressor interface {
	io.Writer

	Flush() error
}

type WriteResetter interface {
	Reset(io.Writer)
}

// Writer contains logic of compressing data into a destination.
// Writer may be reused for different destinations after its Reset() method
// call.
type Writer struct {
	// Dest is a destination of compressed data.
	Dest io.Writer

	// Compressor is a required callback function which must create and
	// configure Compressor instance for given io.Writer.
	//
	// Note that given io.Writer is not the same as the Dest field.
	//
	// If Compressor doesn't implement WriteResetter interface this function
	// will be called on each Reset() call.
	Compressor func(io.Writer) Compressor

	c    Compressor
	err  error
	cbuf cbuf
}

func (w *Writer) init() {
	if w.c == nil {
		w.cbuf.reset(w.Dest)
		w.c = w.Compressor(&w.cbuf)
	}
}

// Reset resets Writer to compress data into dest.
// Any not flushed data will be lost.
func (w *Writer) Reset(dest io.Writer) {
	w.init()
	w.err = nil
	w.cbuf.reset(dest)
	if wr, _ := w.c.(WriteResetter); wr != nil {
		wr.Reset(&w.cbuf)
	} else {
		w.c = w.Compressor(&w.cbuf)
	}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.init()
	if w.err != nil {
		return 0, w.err
	}
	n, w.err = w.c.Write(p)
	return n, w.err
}

// Flush writes any pending data into w.Dest.
func (w *Writer) Flush() error {
	w.init()
	if w.err != nil {
		return w.err
	}
	w.err = w.c.Flush()
	w.checkTail()
	return w.err
}

// Close closes writer and a Compressor instance used under the hood (if it
// implements io.Closer interface).
func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	w.init()
	if c, _ := w.c.(io.Closer); c != nil {
		w.err = c.Close()
	}
	w.checkTail()
	return w.err
}

func (w *Writer) Err() error {
	return w.err
}

func (w *Writer) checkTail() {
	if w.err == nil && w.cbuf.buf != compressionTail {
		w.err = fmt.Errorf(
			"wsflate: bad compressor: unexpected stream tail: %#x vs %#x",
			w.cbuf.buf, compressionTail,
		)
	}
}
