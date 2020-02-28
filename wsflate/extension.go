package wsflate

import (
	"fmt"

	"github.com/gobwas/ws"
)

var errNonFirstFragmentEnabledBit = ws.ProtocolError(
	"non-first fragment contains compression bit enabled",
)

// ExtendRead changes RSV bits of the received frame header as if compression
// extension was negotiated.
func ExtendRead(fseq int, rsv byte) (byte, error) {
	r1, r2, r3 := ws.RsvBits(rsv)
	if fseq > 0 && r1 {
		// An endpoint MUST NOT set the "Per-Message Compressed"
		// bit of control frames and non-first fragments of a data
		// message. An endpoint receiving such a frame MUST _Fail
		// the WebSocket Connection_.
		return rsv, errNonFirstFragmentEnabledBit
	}
	if fseq > 0 {
		return rsv, nil
	}
	return ws.Rsv(false, r2, r3), nil
}

// ExtendRead changes RSV bits of the frame header which is being send as if
// compression extension was negotiated.
func ExtendWrite(fseq int, rsv byte) (byte, error) {
	r1, r2, r3 := ws.RsvBits(rsv)
	if r1 {
		return rsv, fmt.Errorf("wsflate: compression bit is already set")
	}
	if fseq > 0 {
		// An endpoint MUST NOT set the "Per-Message Compressed"
		// bit of control frames and non-first fragments of a data
		// message. An endpoint receiving such a frame MUST _Fail
		// the WebSocket Connection_.
		return rsv, nil
	}
	return ws.Rsv(true, r2, r3), nil
}
