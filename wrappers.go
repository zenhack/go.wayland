package wayland

import (
	"io"
)

type typeWrapper_new_id ObjectId
type typeWrapper_int int32
type typeWrapper_uint uint32
type typeWrapper_Fixed Fixed
type typeWrapper_fd int
type typeWrapper_string string

func (typeWrapper_new_id) Size() uint16 { return 4 }
func (typeWrapper_int) Size() uint16    { return 4 }
func (typeWrapper_uint) Size() uint16   { return 4 }
func (typeWrapper_Fixed) Size() uint16  { return 4 }
func (typeWrapper_fd) Size() uint16     { return 0 }

func (s typeWrapper_string) Size() uint16 {
	return 4 + ceil32(len(s)+1)
}

func writeU32(w io.Writer, val uint32) (int64, error) {
	var buf [4]byte
	hostEndian.PutUint32(buf[:], val)
	n, err := w.Write(buf)
	return int64(n), err
}

func (v typeWrapper_new_id) WriteTo(w io.Writer) (n int64, err error) {
	return writeU32(w, uint32(v))
}

func (v typeWrapper_int) WriteTo(w io.Writer) (n int64, err error) {
	return writeU32(w, uint32(v))
}

func (v typeWrapper_uint) WriteTo(w io.Writer) (n int64, err error) {
	return writeU32(w, uint32(v))
}

func (v typeWrapper_Fixed) WriteTo(w io.Writer) (n int64, err error) {
	return writeU32(w, uint32(v))
}

func (typeWrapper_fd) WriteTo(w io.Writer) (int64, error) {
	return 0, nil
}

func (s typeWrapper_string) WriteTo(w io.Writer) (n int64, err error) {
	n, err = writeU32(w, uint32(len(s)))
	if err != nil {
		return
	}
	n_, err := w.Write([]byte(s))
	n += uint64(n_)
	if err != nil {
		return
	}
	padding := ceil32(len(s)+1) - len(s)
	n_, err = w.Write(buf32[:padding])
	n += uint64(n_)
	return
}
