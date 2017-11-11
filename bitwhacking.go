package wayland

// Round n up to the nearest multiple of 4 (32-bit boundary in bytes).
func ceil32(n int) int {
	return (n + 3) & ^0x3
}
