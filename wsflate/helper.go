package wsflate

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/gobwas/ws"
)

// DefaultHelper is a default helper instance holding standard library's
// `compress/flate` compressor and decompressor under the hood.
//
// Note that use of DefaultHelper methods assumes that DefaultParameters were
// used for extension negotiation during WebSocket handshake.
var DefaultHelper = Helper{
	Compressor: func(w io.Writer) Compressor {
		f, _ := flate.NewWriter(w, 9)
		return f
	},
	Decompressor: func(r io.Reader) Decompressor {
		return flate.NewReader(r)
	},
}

// DefaultParameters holds deflate extension parameters which are assumed by
// DefaultHelper to be used during WebSocket handshake.
var DefaultParameters = Parameters{
	ServerNoContextTakeover: true,
	ClientNoContextTakeover: true,
}

// CompressFrame is a shortcut for DefaultHelper.CompressFrame().
//
// Note that use of DefaultHelper methods assumes that DefaultParameters were
// used for extension negotiation during WebSocket handshake.
func CompressFrame(f ws.Frame) (ws.Frame, error) {
	return DefaultHelper.CompressFrame(f)
}

// CompressFrameBuffer is a shortcut for DefaultHelper.CompressFrameBuffer().
//
// Note that use of DefaultHelper methods assumes that DefaultParameters were
// used for extension negotiation during WebSocket handshake.
func CompressFrameBuffer(buf Buffer, f ws.Frame) (ws.Frame, error) {
	return DefaultHelper.CompressFrameBuffer(buf, f)
}

// DecompressFrame is a shortcut for DefaultHelper.DecompressFrame().
//
// Note that use of DefaultHelper methods assumes that DefaultParameters were
// used for extension negotiation during WebSocket handshake.
func DecompressFrame(f ws.Frame) (ws.Frame, error) {
	return DefaultHelper.DecompressFrame(f)
}

// DecompressFrameBuffer is a shortcut for
// DefaultHelper.DecompressFrameBuffer().
//
// Note that use of DefaultHelper methods assumes that DefaultParameters were
// used for extension negotiation during WebSocket handshake.
func DecompressFrameBuffer(buf Buffer, f ws.Frame) (ws.Frame, error) {
	return DefaultHelper.DecompressFrameBuffer(buf, f)
}

// Helper is a helper struct that holds common code for compressing and
// decompressing bytes or WebSocket frames.
//
// Its purpose is to reduce boilerplate code in WebSocket applications.
type Helper struct {
	Compressor   func(w io.Writer) Compressor
	Decompressor func(r io.Reader) Decompressor
}

// Buffer is an interface representing some bytes buffering object.
type Buffer interface {
	io.Writer
	Bytes() []byte
}

// CompressFrame returns compressed version of a frame.
// Note that it does memory allocations internally. To control those
// allocations consider using CompressFrameBuffer().
func (h *Helper) CompressFrame(in ws.Frame) (f ws.Frame, err error) {
	var buf bytes.Buffer
	return h.CompressFrameBuffer(&buf, in)
}

// DecompressFrame returns decompressed version of a frame.
// Note that it does memory allocations internally. To control those
// allocations consider using DecompressFrameBuffer().
func (h *Helper) DecompressFrame(in ws.Frame) (f ws.Frame, err error) {
	var buf bytes.Buffer
	return h.DecompressFrameBuffer(&buf, in)
}

// CompressFrameBuffer compresses a frame using given buffer.
// Returned frame's payload holds bytes returned by buf.Bytes().
func (h *Helper) CompressFrameBuffer(buf Buffer, in ws.Frame) (f ws.Frame, err error) {
	if !in.Header.Fin {
		return f, fmt.Errorf("wsflate: fragmented messages are not allowed")
	}
	p, err := h.CompressBuffer(buf, in.Payload)
	if err != nil {
		return f, err
	}
	// Copy initial frame.
	f = in
	f.Payload = p
	f.Header.Length = int64(len(p))
	f.Header.Rsv, err = BitsSend(0, f.Header.Rsv)
	if err != nil {
		return f, err
	}
	return f, nil
}

// DecompressFrameBuffer decompresses a frame using given buffer.
// Returned frame's payload holds bytes returned by buf.Bytes().
func (h *Helper) DecompressFrameBuffer(buf Buffer, in ws.Frame) (f ws.Frame, err error) {
	if !in.Header.Fin {
		return f, fmt.Errorf("wsflate: fragmented messages are not allowed")
	}
	p, err := h.DecompressBuffer(buf, in.Payload)
	if err != nil {
		return f, err
	}
	// Copy initial frame.
	f = in
	f.Payload = p
	f.Header.Length = int64(len(p))
	f.Header.Rsv, err = BitsRecv(0, f.Header.Rsv)
	if err != nil {
		return f, err
	}
	return f, nil
}

// Compress compresses given bytes.
// Note that it does memory allocations internally. To control those
// allocations consider using CompressBuffer().
func (h *Helper) Compress(p []byte) ([]byte, error) {
	var buf bytes.Buffer
	return h.CompressBuffer(&buf, p)
}

// Decompress decompresses given bytes.
// Note that it does memory allocations internally. To control those
// allocations consider using DecompressBuffer().
func (h *Helper) Decompress(p []byte) ([]byte, error) {
	var buf bytes.Buffer
	return h.DecompressBuffer(&buf, p)
}

// CompressBuffer compresses bytes using given buffer.
// Returned bytes are bytes returned by buf.Bytes().
func (h *Helper) CompressBuffer(buf Buffer, p []byte) (_ []byte, err error) {
	w := NewWriter(buf, h.Compressor)
	if _, err = w.Write(p); err != nil {
		return nil, err
	}
	if err = w.Flush(); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecompressBuffer decompresses bytes using given buffer.
// Returned bytes are bytes returned by buf.Bytes().
func (h *Helper) DecompressBuffer(buf Buffer, p []byte) (_ []byte, err error) {
	fr := NewReader(bytes.NewReader(p), h.Decompressor)
	if _, err = io.Copy(buf, fr); err != nil {
		return nil, err
	}
	if err = fr.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
