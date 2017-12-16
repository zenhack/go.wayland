package wayland

import (
	"io"
)

func readU32(offset *int, buf []byte) (uint32, error) {
	if *offset+4 > len(buf) {
		return 0, io.ErrUnexpectedEOF
	}
	ret := hostEndian.Uint32(buf[*offset : *offset+4])
	*offset += 4
	return ret, nil
}

func read_new_id(offset *int, buf []byte) (ObjectId, error) {
	ret, err := readU32(offset, buf)
	return ObjectId(ret), err
}

func read_int(offset *int, buf []byte) (int32, error) {
	ret, err := readU32(offset, buf)
	return int32(ret), err
}

func read_uint(offset *int, buf []byte) (uint32, error) {
	return readU32(offset, buf)
}

func read_fixed(offset *int, buf []byte) (Fixed, error) {
	ret, err := readU32(offset, buf)
	return Fixed{value: ret}, err
}

func read_object(offset *int, buf []byte) (ObjectId, error) {
	ret, err := readU32(offset, buf)
	return ObjectId(ret), err
}

func read_string(offset *int, buf []byte) (string, error) {
	size32, err := readU32(offset, buf)
	if err != nil {
		return "", err
	}
	size := int(size32)
	if *offset+ceil32(size+1) > len(buf) {
		return "", io.ErrUnexpectedEOF
	}
	if buf[*offset+size] != 0 {
		return "", ErrMissingNul
	}
	ret := string(buf[*offset : size-1])
	*offset += ceil32(size + 1)
	return ret, nil
}

func read_array(offset *int, buf []byte) ([]byte, error) {
	// TODO: this has too mch in common with read_string;
	// factor some of it out.
	size32, err := readU32(offset, buf)
	if err != nil {
		return nil, err
	}
	size := int(size32)
	if *offset+ceil32(size) > len(buf) {
		return nil, io.ErrUnexpectedEOF
	}
	ret := buf[*offset:size]
	*offset += ceil32(size)
	return ret, err
}
