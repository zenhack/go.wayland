package wayland

import (
	"io"
)

func readU32(offset *int, buf []byte) (uint32, error) {
	if *offset+4 > len(buf) {
		return io.ErrUnexpectedEOF
	}
	ret := hostEndian.Uint32(buf[*offset : *offset+4])
	*offset += 4
	return ret, nil
}

func read_string(offset *int, buf []byte) (string, error) {
	size, err := readU32(offset, buf)
	if err != nil {
		return "", err
	}
	if *offset+ceil32(size+1) > len(buf) {
		return io.ErrUnexpectedEOF
	}
	if buf[*offset+size] != 0 {
		return ErrMissingNul
	}
	return buf[*offset : size-1]
}
