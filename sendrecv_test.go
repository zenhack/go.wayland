package wayland

import (
	"golang.org/x/sys/unix"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

const (
	testMessage = "hello"
)

// Test Conn.send and Conn.recv.
func TestSendRecv(t *testing.T) {
	// TEST_SEND_CHILD indicates that we've been spawned as a child by
	// the test suite.
	if os.Getenv("TEST_SEND_CHILD") == "1" {
		testSendRecvChild(t)
	} else {
		testSendRecvParent(t)
	}
}

func connFromFd(t *testing.T, fd int) *Client {
	socket, err := net.FileConn(os.NewFile(uintptr(fd), "socket"))
	if err != nil {
		t.Fatal(err)
	}
	return newClient(socket.(*net.UnixConn))
}

// Child half of TestSendRecv. Accept a message and file descriptor on fd #3,
// then write the received message to the received file descriptor.
func testSendRecvChild(t *testing.T) {
	defer os.Exit(0)

	defer unix.Close(3)
	conn := connFromFd(t, 3)

	// Make the buffer a bit bigger than needed, so if we get more data than
	// expected we catch it:
	buf := make([]byte, len(testMessage)+2)

	// make sure that if this is not overwritten, it can't be confused for
	// a valid fd:
	fds := []int{-1}

	n, nfd, err := conn.recv(buf, fds)
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if n != len(testMessage) || nfd != len(fds) {
		t.Fatalf("Wrong read lengths; expected (%d, %d) but got (%d, %d)",
			len(testMessage), len(fds), n, nfd)
	}
	defer unix.Close(fds[0])
	n, err = os.NewFile(uintptr(fds[0]), "pipe").Write(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testMessage) {
		t.Fatal("Wrong write length; expected", len(testMessage), "but got", n)
	}
}

// Execute the test binary, running only TestSendRecv, setting the
// environment variable "TEST_SEND_CHILD" to 1 so the child knows it's
// the child.
func spawnTestChild(t *testing.T, fd int) (stdout, stderr io.ReadCloser, cmd *exec.Cmd, err error) {
	cmd = exec.Command(os.Args[0], "-test.run", "^TestSendRecv$")
	cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(fd), "socket")}
	cmd.Env = append(os.Environ(), "TEST_SEND_CHILD=1")
	stdout, err = cmd.StdoutPipe()
	if err != nil {
		goto fail_stdout
	}
	stderr, err = cmd.StderrPipe()
	if err != nil {
		goto fail_stderr
	}
	if err = cmd.Start(); err != nil {
		goto fail_start
	}
	return
fail_start:
	stderr.Close()
fail_stderr:
	stdout.Close()
fail_stdout:
	t.Fatal(err)
	// The compiler doesn't seem to know that Fatal never returns:
	panic("unreachable")
}

// Parent half of TestSendRecv. Spawn a child process with an attached socket,
// send the write end of a pipe and a message over the socket, then try to
// read the same message from the pipe.
func testSendRecvParent(t *testing.T) {

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fds[1])

	stdout, stderr, cmd, err := spawnTestChild(t, fds[0])
	unix.Close(fds[0])
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		time.AfterFunc(3*time.Second, func() { cmd.Process.Kill() })
		outbytes, err := ioutil.ReadAll(stdout)
		t.Logf("output from child (error %v): %s", err, outbytes)
		errbytes, err := ioutil.ReadAll(stderr)
		t.Logf("stderr from child (error %v): %s", err, errbytes)
		stdout.Close()
		stderr.Close()
		if err := cmd.Wait(); err != nil {
			t.Fatal("Error from child:", err)
		}
	}()

	conn := connFromFd(t, fds[1])
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	err = conn.send([]byte(testMessage), []int{int(w.Fd())})
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Make the buffer a bit bigger, so we can tell if the child writes
	// more data than expected.
	buf := make([]byte, len(testMessage)+2)
	sz, err := r.Read(buf)
	if sz != len(testMessage) || err != nil {
		t.Fatal("Read", sz, "bytes with error", err, "expected",
			len(testMessage), "bytes and err == nil")
	}
	receivedMessage := string(buf[:sz])
	if receivedMessage != testMessage {
		t.Fatal("Expected", testMessage, "but got", receivedMessage)
	}
}
