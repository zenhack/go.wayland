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
	client, err := wayland.Dial("")
	chkfatal(err)
	display := client.GetDisplay()
	registry, err := display.GetRegistry()
	chkfatal(err)
	registry.OnGlobal(func(name uint32, iface string, version uint32) {
		fmt.Printf("new global: (%d, %s, %d)\n", name, iface, version)
	})
	chkfatal(client.Sync(func() {
		os.Exit(0)
	}))
	chkfatal(client.MainLoop())
}
