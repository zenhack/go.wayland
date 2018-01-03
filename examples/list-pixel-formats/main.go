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
		shm, ok := obj.(*wayland.Shm)
		if !ok {
			return
		}
		fmt.Println("Got the shm object; querying formats.")
		shm.OnFormat(func(format uint32) {
			fmt.Println(format)
		})

		// We don't care about the rest of the globals; unsubscribe:
		client.OnGlobal(nil)

		// exit when the format events are done:
		client.Sync(func() {
			os.Exit(0)
		})
	})
	chkfatal(client.MainLoop())
}
