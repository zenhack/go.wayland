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
	Interface() string
	Version() uint32
}

type hasObjectId interface {
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

// An error returned by the server.
type ServerError struct {
	ObjectId  ObjectId
	ErrorCode uint32
	Message   string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf(
		"Server error: %q (object id = %d, error code = %d)",
		e.Message, e.ObjectId, e.ErrorCode,
	)
}

type UnknownInterface struct {
	id         ObjectId
	interface_ string
	version    uint32
}

func (i *UnknownInterface) Id() ObjectId {
	return i.id
}

func (i *UnknownInterface) Interface() string {
	return i.interface_
}

func (i *UnknownInterface) Version() uint32 {
	return i.version
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

type Client struct {
	lock    sync.Mutex
	socket  *net.UnixConn
	nextId  uint32
	objects map[ObjectId]remoteProxy

	display  *Display
	registry *Registry
	onGlobal func(obj Object)

	// An error received from the server's Display object. if this is set,
	// the next iteration in MainLoop will exit, returning it.
	receivedError error
}

func newClient(uconn *net.UnixConn) *Client {
	ret := &Client{
		socket: uconn,
		nextId: 2,
	}
	ret.objects = map[ObjectId]remoteProxy{1: &Display{
		remoteObject: remoteObject{
			id:   1,
			conn: ret,
		},
	}}
	return ret
}

func guessSocketPath() string {
	return fmt.Sprintf("/var/run/user/%d/wayland-0", os.Getuid())
}

func Dial(path string) (*Client, error) {
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
	client := newClient(uconn)
	client.display = &Display{
		remoteObject: remoteObject{
			id:   1,
			conn: client,
		},
	}
	client.display.OnError(func(oid ObjectId, code uint32, message string) {
		client.receivedError = &ServerError{
			ObjectId:  oid,
			ErrorCode: code,
			Message:   message,
		}
	})
	client.display.OnDeleteId(func(id uint32) {
		delete(client.objects, ObjectId(id))
		// TODO: we probably need to do some bookkeeping to coordinate
		// with nextId().
	})
	client.registry, err = client.display.GetRegistry()
	if err != nil {
		uconn.Close()
		return nil, err
	}
	client.registry.OnGlobal(func(name uint32, interface_ string, version uint32) {
		if client.onGlobal == nil {
			return
		}
		ifaceFn, ok := interfaceRegistry[interfaceIdent{
			Name:    interface_,
			Version: version,
		}]
		if ok {
			id, err := client.registry.Bind(name)
			if err != nil {
				//TODO: better error handling.
				client.receivedError = err
				return
			}
			obj := ifaceFn(client, id)
			client.objects[id] = obj
			client.onGlobal(obj)
		} else {
			client.onGlobal(&UnknownInterface{
				// We don't call Bind, so this has a null id:
				id: 0,

				interface_: interface_,
				version:    version,
			})
		}
	})
	return client, nil
}

func (c *Client) Sync(fn func()) error {
	cb, err := c.display.Sync()
	if err != nil {
		return err
	}
	cb.OnDone(func(uint32) { fn() })
	return nil
}

func (c *Client) GetDisplay() *Display {
	return c.display
}

func (c *Client) GetRegistry() *Registry {
	return c.registry
}

// Send the data and file descriptors over the connection's socket. len(data)
// must not be 0.
func (c *Client) send(data []byte, fds []int) error {
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
func (c *Client) recv(data []byte, fds []int) (n, fdn int, err error) {
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

func (c *Client) nextMsg() error {
	hdr := header{}
	_, err := (&hdr).ReadFrom(c.socket)
	if err != nil {
		return err
	}
	if hdr.Size < 8 {
		return fmt.Errorf("Received message's header specifies a "+
			"size (%d) that is too small (minmum is 8)", hdr.Size)
	}
	sender, ok := c.objects[hdr.Sender]
	if !ok {
		return fmt.Errorf("Unknown object id: %d\n", hdr.Sender)
	}
	events := sender.getFdCounts().events
	if len(events) <= int(hdr.Opcode) {
		return fmt.Errorf("Opcode %d for object %d is out of range",
			hdr.Opcode, hdr.Sender)
	}
	fds := make([]int, events[hdr.Opcode])
	data := make([]byte, hdr.Size-8)
	n, nfds, err := c.recv(data, fds)
	if err != nil {
		closeAll(fds[:nfds])
		return err
	}
	if n != len(data) || nfds != len(fds) {
		// TODO: can we handle this gracefully? Do we need to?
		closeAll(fds[:nfds])
		return fmt.Errorf("Short read")
	}
	sender.handleEvent(hdr.Opcode, data, fds)
	return nil
}

func (c *Client) OnGlobal(callback func(Object)) {
	c.onGlobal = callback
}

func (c *Client) MainLoop() error {
	for {
		if err := c.nextMsg(); err != nil {
			return err
		}
	}
}

// Allocate and return a fresh object id.
func (c *Client) newId() ObjectId {
	ret := c.nextId
	c.nextId++
	return ObjectId(ret)
}

// An object hosted on the other side of a connection.
//
// TODO: pick better names/document the distinction between this and remoteProxy.
type remoteObject struct {
	id   ObjectId
	conn *Client
}

func (o *remoteObject) Id() ObjectId {
	return o.id
}

type remoteProxy interface {
	Object
	getFdCounts() *fdCounts
	handleEvent(opcode uint16, buf []byte, fds []int)
}

// helper function to avoid errors about unused variables in generated code.
func noOpInt(int) {}
