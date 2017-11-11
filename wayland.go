package wayland

//go:generate go run internal/gen/main.go

import (
	"io"
	"sync"
)

// Signed 24.8 decimal numbers. It is a signed decimal type which
// offers a sign bit, 23 bits of integer precision and 8 bits of
// decimal precision.
type Fixed struct {
	value uint32
}

type ObjectId uint32

type Header struct {
	Sender ObjectId
	Opcode uint16
	Size   uint16
}

func (h Header) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	hostEndian.PutUint32(buf[:4], uint32(h.Sender))
	hostEndian.PutUint32(buf[4:], uint32(h.Size)<<16|uint32(h.Opcode))
	n, err := w.Write(buf[:])
	return int64(n), err
}

type Conn struct {
	lock   sync.Mutex
	nextId uint32
}

func (c *Conn) newId() uint32 {
	ret := c.nextId
	c.nextId++
	return ret
}

type remoteObject struct {
	id   ObjectId
	conn *Conn
}
