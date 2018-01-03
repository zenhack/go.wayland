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
	client.OnGlobal(func(obj wayland.Object) {
		fmt.Printf("new global: (%d, %s, %d)\n", obj.Id(), obj.Interface(), obj.Version())
	})
	chkfatal(client.Sync(func() {
		os.Exit(0)
	}))
	chkfatal(client.MainLoop())
}
