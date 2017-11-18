// +build amd64 arm 386 ppc64le mipsle mips64le
package wayland

import (
	"encoding/binary"
)

var hostEndian = binary.LittleEndian
