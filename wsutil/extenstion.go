package wsutil

type Extension interface {
	Extend(frameSeq int, rsv byte) (byte, error)
}

var _ Extension = ExtensionFunc(nil)

type ExtensionFunc func(frameSeq int, rsv byte) (byte, error)

func (fn ExtensionFunc) Extend(fseq int, rsv byte) (byte, error) {
	return fn(fseq, rsv)
}
