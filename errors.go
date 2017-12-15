package wayland

import (
	"errors"
)

var ErrMissingNul = errors.New("String in message body was missing NUL terminator.")
