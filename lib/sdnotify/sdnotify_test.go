package sdnotify

import (
	"crypto/rand"
	"net"
	"testing"
	"time"
)

func TestNotifyReady(t *testing.T) {
	t.Parallel()
	addr := "@lys-sdnotify-" + rand.Text()
	ln, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := Notify(addr, "READY=1"); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if err := ln.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, err := ln.Read(buf)
	if err != nil || string(buf[:n]) != "READY=1" {
		t.Errorf("the supervisor read %q, %v", buf[:n], err)
	}
	if err := Notify("", "READY=1"); err != nil {
		t.Errorf("Notify without a supervisor: %v", err)
	}
	if err := Notify("@lys-sdnotify-nobody-"+rand.Text(), "READY=1"); err == nil {
		t.Error("Notify to a missing socket succeeded")
	}
}
