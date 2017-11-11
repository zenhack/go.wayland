// +build amd64 arm 386
package wayland

import (
	"encoding/binary"
)

var hostEndian = binary.LittleEndian
