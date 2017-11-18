// +build ppc64be mipsbe mips64be

package wayland

import (
	"encoding/binary"
)

var hostEndian = binary.BigEndian
