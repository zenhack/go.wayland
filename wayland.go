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

// A side of the connection (server or client).
type side int

const minServerId = 0xff000000

const (
	clientSide side = iota
	serverSide
)

// Signed 24.8 decimal numbers. It is a signed decimal type which
// offers a sign bit, 23 bits of integer precision and 8 bits of
// decimal precision.
type Fixed struct {
	value uint32
}

type ObjectId uint32

// Return which side of the connection the object lives in.
func (o ObjectId) home() side {
	if o >= 1 && o < minServerId {
		return clientSide
	}
	return serverSide
}

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
	objects map[ObjectId]remoteProxy
	side    side
}

func newConn(side side, uconn *net.UnixConn) *Conn {
	ret := &Conn{
		side:   side,
		socket: uconn,
	}
	ret.objects = map[ObjectId]remoteProxy{0: &remoteDisplay{
		remoteObject: remoteObject{
			id:   0,
			conn: ret,
		},
	}}
	switch side {
	case clientSide:
		ret.nextId = 1
	case serverSide:
		ret.nextId = minServerId
	default:
		panic("Invalid connection side!")
	}
	return ret
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
	return newConn(clientSide, uconn), nil
}

func (c *Conn) GetDisplay() Display {
	return &remoteDisplay{
		remoteObject: remoteObject{
			id:   0,
			conn: c,
		},
	}
}

// Send the data and file descriptors over the connection's socket. len(data)
// must not be 0.
func (c *Conn) send(data []byte, fds []int) error {
	_, _, err := c.socket.WriteMsgUnix(data, unix.UnixRights(fds...), nil)
	return err
}

func closeAll(fds []int) {
	for _, fd := range fds {
		unix.Close(fd)
	}
}

// Read data and file descriptors from the connection. `n` indicates the number
// of bytes that were read, and `fdn` indicates the number of file descriptors
// that were read.
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
	for _, cmsg := range cmsgs {
		msgFds, errParse := unix.ParseUnixRights(&cmsg)
		if errParse != nil {
			closeAll(fdsRecv)
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

func (c *Conn) readMsg() (data []byte, fds []int, err error) {
	if c.side == serverSide {
		panic("TODO: handle server side.")
	}
	hdr := header{}
	_, err = (&hdr).ReadFrom(c.socket)
	if err != nil {
		return
	}
	if hdr.Size < 8 {
		err = fmt.Errorf("Received message's header specifies a "+
			"size (%d) that is too small (minmum is 8)", hdr.Size)
		return
	}
	sender, ok := c.objects[hdr.Sender]
	if !ok {
		err = fmt.Errorf("Unknown object id: %d\n", hdr.Sender)
		return
	}
	if hdr.Sender.home() == serverSide {
		events := sender.getFdCounts().events
		if len(events) <= int(hdr.Opcode) {
			err = fmt.Errorf("Opcode %d for object %d is out of range",
				hdr.Opcode, hdr.Sender)
			return
		}
		fds = make([]int, events[hdr.Opcode])
	} else {
		panic("TODO: figure out what to do on the client")
	}
	data = make([]byte, hdr.Size-8)
	n, nfds, err := c.recv(data, fds)
	if err != nil {
		closeAll(fds[:nfds])
		return
	}
	if n != len(data) || nfds != len(fds) {
		// TODO: can we handle this gracefully? Do we need to?
		closeAll(fds[:nfds])
		err = fmt.Errorf("Short read")
	}
	return
}

// Allocate and return a fresh object id.
func (c *Conn) newId() ObjectId {
	ret := c.nextId
	c.nextId++
	return ObjectId(ret)
}

// An object hosted on the other side of a connection.
//
// TODO: pick better names/document the distinction between this and remoteProxy.
type remoteObject struct {
	id   ObjectId
	conn *Conn
}

func (o *remoteObject) Id() ObjectId {
	return o.id
}

type remoteProxy interface {
	getFdCounts() *fdCounts
	handleEvent(opcode uint16, buf []byte, fds []int)
}

// helper function to avoid errors about unused variables in generated code.
func noOpInt(int) {}
