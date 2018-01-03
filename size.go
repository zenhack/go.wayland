package wayland

func sizeOf_new_id(hasObjectId) uint16 { return 4 }
func sizeOf_int(int32) uint16          { return 4 }
func sizeOf_uint(uint32) uint16        { return 4 }
func sizeOf_Fixed(Fixed) uint16        { return 4 }
func sizeOf_object(Object) uint16      { return 4 }
func sizeOf_fd(int) uint16             { return 0 }

func sizeOf_string(s string) uint16 {
	// XXX: we need to make sure this doesn't overflow somehow.
	return uint16(4 + ceil32(len(s)+1))
}
