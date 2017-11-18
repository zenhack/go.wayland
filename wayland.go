package wayland

//go:generate go run internal/gen/main.go

import (
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"net"
	"os"
	"sync"
)

// Signed 24.8 decimal numbers. It is a signed decimal type which
// offers a sign bit, 23 bits of integer precision and 8 bits of
// decimal precision.
type Fixed struct {
	value uint32
}

type ObjectId uint32

type Object interface {
	Id() ObjectId
}

type Header struct {
	Sender ObjectId
	Opcode uint16
	Size   uint16
}

// mapping from an interface's opcodes to the number of file descriptor
// arguments for the corresponding request or event.
type fdCounts struct {
	requests, events []int
}

func (h Header) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	hostEndian.PutUint32(buf[:4], uint32(h.Sender))
	hostEndian.PutUint32(buf[4:], uint32(h.Size)<<16|uint32(h.Opcode))
	n, err := w.Write(buf[:])
	return int64(n), err
}

func (h *Header) ReadFrom(r io.Reader) (int64, error) {
	var buf [8]byte
	n, err := io.ReadFull(r, buf[:])
	if err != nil {
		return int64(n), err
	}
	opcodeAndSize := hostEndian.Uint32(buf[4:])
	*h = Header{
		Sender: ObjectId(hostEndian.Uint32(buf[:4])),
		Opcode: uint16(opcodeAndSize),
		Size:   uint16(opcodeAndSize >> 16),
	}
	return int64(n), nil
}

func ReadMessage(conn *Conn) (Header, []byte, error) {
	conn.lock.Lock()
	defer conn.lock.Unlock()
	header := Header{}
	_, err := (&header).ReadFrom(conn.socket)
	if err != nil {
		return Header{}, nil, err
	}
	buf := make([]byte, header.Size)
	_, err = io.ReadFull(conn.socket, buf)
	return header, buf, err
}

type Conn struct {
	lock   sync.Mutex
	addr   *net.UnixAddr
	socket *net.UnixConn
	nextId uint32
}

func guessSocketPath() string {
	return fmt.Sprintf("/var/run/user/%d/wayland-0", os.Getuid())
}

func Dial(path string) (*Conn, error) {
	if path == "" {
		path = guessSocketPath()
	}
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		return nil, err
	}
	uconn, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return nil, err
	}
	return &Conn{
		addr:   addr,
		socket: uconn,
		nextId: 1,
	}, nil
}

func (c *Conn) send(data []byte, fds []int) error {
	_, _, err := c.socket.WriteMsgUnix(data, unix.UnixRights(fds...), c.addr)
	return err
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
