package main

import (
	"fmt"
	"os"

	"zenhack.net/go/wayland"
)

func chkfatal(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	conn, err := wayland.Dial("")
	chkfatal(err)
	display := conn.GetDisplay()
	display.OnError(func(oid wayland.ObjectId, code uint32, message string) {
		fmt.Printf("error from server: (%d, %d, %q)\n", oid, code, message)
		os.Exit(1)
	})
	registry, err := display.GetRegistry()
	chkfatal(err)
	registry.OnGlobal(func(name uint32, iface string, version uint32) {
		fmt.Printf("new global: (%d, %s, %d)\n", name, iface, version)
	})
	cb, err := display.Sync()
	chkfatal(err)
	cb.OnDone(func(uint32) {
		os.Exit(0)
	})
	chkfatal(conn.MainLoop())
}
