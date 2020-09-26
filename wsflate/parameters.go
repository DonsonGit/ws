package wsflate

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/gobwas/httphead"
)

const (
	ExtensionName = "permessage-deflate"

	serverNoContextTakeover = "server_no_context_takeover"
	clientNoContextTakeover = "client_no_context_takeover"
	serverMaxWindowBits     = "server_max_window_bits"
	clientMaxWindowBits     = "client_max_window_bits"
)

var (
	ExtensionNameBytes = []byte(ExtensionName)

	serverNoContextTakeoverBytes = []byte(serverNoContextTakeover)
	clientNoContextTakeoverBytes = []byte(clientNoContextTakeover)
	serverMaxWindowBitsBytes     = []byte(serverMaxWindowBits)
	clientMaxWindowBitsBytes     = []byte(clientMaxWindowBits)

	windowBits [8][]byte
)

func isValidBits(x int) bool {
	return 8 <= x && x <= 15
}

func init() {
	for i := range windowBits {
		windowBits[i] = []byte(strconv.Itoa(i + 8))
	}
}

// Parameters contains compressin extension options.
type Parameters struct {
	ServerNoContextTakeover bool
	ClientNoContextTakeover bool
	ServerMaxWindowBits     WindowBits
	ClientMaxWindowBits     WindowBits
}

// WindowBits specifies window size accordingly to RFC.
// Use its Bytes() method to obtain actual size of window.
type WindowBits byte

// Defined reports whether window bits were specified.
func (b WindowBits) Defined() bool {
	return b > 0
}

// Bytes returns window size in number of bytes.
func (b WindowBits) Bytes() int {
	return 1 << uint(b)
}

func bitsFromASCII(p []byte) (WindowBits, bool) {
	n, ok := httphead.IntFromASCII(p)
	if !ok || !isValidBits(n) {
		return 0, false
	}
	return WindowBits(n), true
}

func setBits(opt *httphead.Option, name []byte, bits WindowBits) {
	if bits == 0 {
		return
	}
	if bits == 1 {
		opt.Parameters.Set(name, nil)
		return
	}
	if !isValidBits(int(bits)) {
		panic(fmt.Sprintf("wsflate: invalid bits value: %d", bits))
	}
	opt.Parameters.Set(name, windowBits[bits-8])
}

func setBool(opt *httphead.Option, name []byte, flag bool) {
	if flag {
		opt.Parameters.Set(name, nil)
	}
}

const (
	MaxLZ77WindowSize = 32768
)

func paramError(reason string, key, val []byte) error {
	return fmt.Errorf(
		"wsflate: %s extension parameter %q: %q",
		reason, key, val,
	)
}

// Parse reads parameters from given HTTP header opiton accordingly to RFC.
//
// It returns non-nil error at least in these cases:
//   - The negotiation offer contains an extension parameter not defined for
//   use in an offer/response.
//   - The negotiation offer/response contains an extension parameter with an
//   invalid value.
//   - The negotiation offer/response contains multiple extension parameters
//   with
// the same name.
func (p *Parameters) Parse(opt httphead.Option) (err error) {
	const (
		clientMaxWindowBitsSeen = 1 << iota
		serverMaxWindowBitsSeen
		clientNoContextTakeoverSeen
		serverNoContextTakeoverSeen
	)

	// Reset to not mix parsed data with previously parsed values.
	*p = Parameters{}

	var seen byte
	opt.Parameters.ForEach(func(key, val []byte) (ok bool) {
		switch string(key) {
		case clientMaxWindowBits:
			if len(val) == 0 {
				p.ClientMaxWindowBits = 1
				return true
			}
			if seen&clientMaxWindowBitsSeen != 0 {
				err = paramError("duplicate", key, val)
				return false
			}
			seen |= clientMaxWindowBitsSeen
			if p.ClientMaxWindowBits, ok = bitsFromASCII(val); !ok {
				err = paramError("invalid", key, val)
				return false
			}

		case serverMaxWindowBits:
			if len(val) == 0 {
				err = paramError("invalid", key, val)
				return false
			}
			if seen&serverMaxWindowBitsSeen != 0 {
				err = paramError("duplicate", key, val)
				return false
			}
			seen |= serverMaxWindowBitsSeen
			if p.ServerMaxWindowBits, ok = bitsFromASCII(val); !ok {
				err = paramError("invalid", key, val)
				return false
			}

		case clientNoContextTakeover:
			if len(val) > 0 {
				err = paramError("invalid", key, val)
				return false
			}
			if seen&clientNoContextTakeoverSeen != 0 {
				err = paramError("duplicate", key, val)
				return false
			}
			seen |= clientNoContextTakeoverSeen
			p.ClientNoContextTakeover = true

		case serverNoContextTakeover:
			if len(val) > 0 {
				err = paramError("invalid", key, val)
				return false
			}
			if seen&serverNoContextTakeoverSeen != 0 {
				err = paramError("duplicate", key, val)
				return false
			}
			seen |= serverNoContextTakeoverSeen
			p.ServerNoContextTakeover = true

		default:
			err = paramError("unexpected", key, val)
			return false
		}
		return true
	})
	return
}

// Option encodes parameters into HTTP header option.
func (p Parameters) Option() httphead.Option {
	opt := httphead.Option{
		Name: ExtensionNameBytes,
	}
	setBool(&opt, serverNoContextTakeoverBytes, p.ServerNoContextTakeover)
	setBool(&opt, clientNoContextTakeoverBytes, p.ClientNoContextTakeover)
	setBits(&opt, serverMaxWindowBitsBytes, p.ServerMaxWindowBits)
	setBits(&opt, clientMaxWindowBitsBytes, p.ClientMaxWindowBits)
	return opt
}

// Extension contains logic of compression extension parameters negotiation.
// It might be reused between different upgrades with Reset() being called
// after each.
type Extension struct {
	// Parameters is specification of extension parameters server is going to
	// accept.
	Parameters Parameters

	accepted bool
	params   Parameters
}

// Negotiate parses given HTTP header option and returns (if any) header option
// which describes accepted parameters.
//
// It may return zero option (i.e. one which Name field is nil) alongside with
// nil error.
func (n *Extension) Negotiate(opt httphead.Option) (accept httphead.Option, err error) {
	if !bytes.Equal(opt.Name, ExtensionNameBytes) {
		return
	}
	if n.accepted {
		// Negotiate might be called multiple times during upgrade.
		// We stick to first one accepted extension since they must be passed
		// in ordered by preference.
		return
	}

	want := n.Parameters

	if err = n.params.Parse(opt); err != nil {
		return
	}
	{
		offer := n.params.ServerMaxWindowBits
		want := want.ServerMaxWindowBits
		if offer > want {
			// A server declines an extension negotiation offer
			// with this parameter if the server doesn't support
			// it.
			return
		}
	}
	{
		// If a received extension negotiation offer has the
		// "client_max_window_bits" extension parameter, the server MAY
		// include the "client_max_window_bits" extension parameter in the
		// corresponding extension negotiation response to the offer.
		offer := n.params.ClientMaxWindowBits
		want := want.ClientMaxWindowBits
		if want > offer {
			return
		}
	}
	{
		offer := n.params.ServerNoContextTakeover
		want := want.ServerNoContextTakeover
		if offer && !want {
			return
		}
	}

	n.accepted = true

	return want.Option(), nil
}

// Accepted returns parameters parsed during last negotiation and a flag that
// reports whether they were accepted.
func (n *Extension) Accepted() (_ Parameters, accepted bool) {
	return n.params, n.accepted
}

// Reset resets extension for further reuse.
func (n *Extension) Reset() {
	n.accepted = false
	n.params = Parameters{}
}
