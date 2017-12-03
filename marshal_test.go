package wayland

import (
	"bytes"
	"testing"
	"testing/quick"
)

// Test header's ReadFrom and WriteTo methods against each other.
func TestHeaderMarshal(t *testing.T) {
	err := quick.Check(func(h header) bool {
		buf := &bytes.Buffer{}
		n, err := h.WriteTo(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n != 8 {
			t.Fatal("Error: WriteTo: header should always be 8 bytes.")
		}
		newH := header{}
		n, err = (&newH).ReadFrom(buf)
		if err != nil {
			t.Fatal(err)
		}
		if n != 8 {
			t.Fatal("Error: ReadFrom: header should always be 8 bytes.")
		}
		if h != newH {
			t.Log("Error: headers differ. Wrote", h, "but read", newH)
			return false
		}
		return true
	}, nil)
	if err != nil {
		t.Error(err)
	}
}
