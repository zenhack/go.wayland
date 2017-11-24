package wayland

import (
	"testing"
	"testing/quick"
)

func TestCeil32(t *testing.T) {
	props := map[string]func(val int) bool{
		"Rounds up": func(val int) bool {
			return ceil32(val) >= val
		},
		"Aligns to 4": func(val int) bool {
			return ceil32(val)%4 == 0
		},
		"Minimal": func(val int) bool {
			return ceil32(val)-4 < val
		},
	}
	for name, pred := range props {
		if err := quick.Check(pred, nil); err != nil {
			t.Error("Property", name, ":", err)
		}
	}
}
