package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/justincormack/go-memfd"

	"zenhack.net/go/wayland"
)

var (
	imgPath = flag.String("img", "", "Path to image to show")
)

func chkfatal(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()
	file, err := os.Open(*imgPath)
	chkfatal(err)
	img, _, err := image.Decode(file)
	chkfatal(err)
	file.Close()

	var (
		shm        *wayland.Shm
		compositor *wayland.Compositor
	)
	client, err := wayland.Dial("")
	chkfatal(err)
	client.OnGlobal(func(obj wayland.Object) {
		switch o := obj.(type) {
		case *wayland.Shm:
			shm = o
		case *wayland.Compositor:
			compositor = o
		default:
			// Don't care.
		}
	})
	chkfatal(client.Sync(func() {
		if compositor == nil {
			fmt.Println("Didn't receive needed compositor object; exiting.")
			os.Exit(1)
		}
		if shm == nil {
			fmt.Println("Didn't receive needed shm object; exiting.")
			os.Exit(1)
		}

		// Unsubscribe to globals:
		client.OnGlobal(nil)

		mfd, err := memfd.Create()
		chkfatal(err)
		bounds := img.Bounds()

		// TODO: handle color model.
		size := bounds.Dx() * bounds.Dy()
		chkfatal(mfd.Truncate(int64(size)))
		mfdBytes, err := mfd.Map()
		chkfatal(err)
		pool, err := shm.CreatePool(int(mfd.Fd()), size)
		chkfatal(err)
		buf, err := pool.CreateBuffer(0, width, height /* TODO: stride, format */)
		chkfatal(err)
	}))
	chkfatal(client.MainLoop())
}
