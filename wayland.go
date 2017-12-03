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

func (o ObjectId) Id() ObjectId {
	return o
}

type Object interface {
	Id() ObjectId
}

type header struct {
	Sender ObjectId
	Opcode uint16
	Size   uint16
}

// mapping from an interface's opcodes to the number of file descriptor
// arguments for the corresponding request or event.
type fdCounts struct {
	requests, events []int
}

func (h header) WriteTo(w io.Writer) (int64, error) {
	var buf [8]byte
	hostEndian.PutUint32(buf[:4], uint32(h.Sender))
	hostEndian.PutUint32(buf[4:], uint32(h.Size)<<16|uint32(h.Opcode))
	n, err := w.Write(buf[:])
	return int64(n), err
}

func (h *header) ReadFrom(r io.Reader) (int64, error) {
	var buf [8]byte
	n, err := io.ReadFull(r, buf[:])
	if err != nil {
		return int64(n), err
	}
	opcodeAndSize := hostEndian.Uint32(buf[4:])
	*h = header{
		Sender: ObjectId(hostEndian.Uint32(buf[:4])),
		Opcode: uint16(opcodeAndSize),
		Size:   uint16(opcodeAndSize >> 16),
	}
	return int64(n), nil
}

type Conn struct {
	lock    sync.Mutex
	socket  *net.UnixConn
	nextId  uint32
	objects map[uint32]*fdCounts
}

func newConn(firstId uint32, uconn *net.UnixConn) *Conn {
	return &Conn{
		socket:  uconn,
		nextId:  firstId,
		objects: map[uint32]*fdCounts{0: &displayFdCounts},
	}
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
	return newConn(1, uconn), nil
}

func (c *Conn) GetDisplay() Display {
	return &remoteDisplay{
		remoteObject: remoteObject{
			id:   0,
			conn: c,
		},
	}
}

func (c *Conn) send(data []byte, fds []int) error {
	_, _, err := c.socket.WriteMsgUnix(data, unix.UnixRights(fds...), nil)
	return err
}

func (c *Conn) recv(data []byte, fds []int) (n, fdn int, err error) {
	oob := make([]byte, unix.CmsgSpace(len(fds)*4))
	n, oobn, _, _, errRead := c.socket.ReadMsgUnix(data, oob)

	// Keep going, even if errRead != nil. This is designed to deal with the
	// situation where we've gotten a short read, receiving some file descriptors
	// in spite of the error.  We should close the fds in this case, to avoid
	// leaking them. We rely on ReadMsgUnix to correctly report the lengths,
	// so if there is a short read that results in an invalid message, it won't
	// parse.
	//
	// TODO: I(zenhack) am not sure the above can actually happen; it would
	// be nice to investigate and, if safe, simplify this.
	firstErr := func(e1, e2 error) error {
		if e1 != nil {
			return e1
		}
		return e2
	}

	cmsgs, errParse := unix.ParseSocketControlMessage(oob[:oobn])
	if errParse != nil {
		return n, 0, firstErr(errRead, errParse)
	}

	fdsRecv := []int{}
	closeAll := func() {
		for _, fd := range fdsRecv {
			unix.Close(fd)
		}
	}
	for _, cmsg := range cmsgs {
		msgFds, errParse := unix.ParseUnixRights(&cmsg)
		if errParse != nil {
			closeAll()
			return n, 0, firstErr(errRead, errParse)
		}
		fdsRecv = append(fdsRecv, msgFds...)
	}
	fdn = len(fdsRecv)
	if len(fdsRecv) > len(fds) {
		// This should never happen; we allocated a buffer that was
		// suposed to be the right size for len(fds) file descriptors,
		// and no more.
		panic("impossible")
	}
	copy(fds[:fdn], fdsRecv)
	return n, fdn, nil
}

func (c *Conn) newId() ObjectId {
	ret := c.nextId
	c.nextId++
	return ObjectId(ret)
}

type remoteObject struct {
	id   ObjectId
	conn *Conn
}

func (o *remoteObject) Id() ObjectId {
	return o.id
}
