package fprint

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const activatedFprintdEnv = "LYS_FPRINT_TEST_ACTIVATED_FPRINTD"

func serveActivatedFprintd() int {
	conn, err := dbustest.StarterConn()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer func() { _ = conn.Close() }()
	dev := &fakeDevice{conn: conn, fingers: []string{"right-index-finger"}, started: make(chan struct{}, 8)}
	if err := conn.Export(&fakeManager{}, ManagerPath, ManagerIface); err != nil {
		fmt.Fprintln(os.Stderr, "fprint: export the activated manager:", err)
		return 1
	}
	if err := conn.Export(dev, devicePath, DeviceIface); err != nil {
		fmt.Fprintln(os.Stderr, "fprint: export the activated device:", err)
		return 1
	}
	if reply, err := conn.RequestName(BusName, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		fmt.Fprintf(os.Stderr, "fprint: own %s: %v %v\n", BusName, reply, err)
		return 1
	}
	for {
		select {
		case <-conn.Context().Done():
			return 0
		case <-dev.started:
			if err := verifyStatusSignal.Emit(conn, devicePath, verifyStatus{Result: "verify-match", Done: true}); err != nil {
				fmt.Fprintln(os.Stderr, "fprint: emit a match:", err)
				return 1
			}
		}
	}
}

func TestAnIdleFprintdIsActivatedToVerify(t *testing.T) {
	t.Setenv(activatedFprintdEnv, "1")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bus := dbustest.Start(t, dbustest.Service{Name: BusName, Exec: []string{exe}})
	results := make(chan Result, 8)
	ended := make(chan error, 1)
	go func() {
		ended <- Verify(context.Background(), bus.Conn(t), "ada", func(r Result) { results <- r })
	}()
	if err := waittest.For(t, ended, "Verify to end"); err != nil {
		t.Fatalf("Verify with an activatable fprintd: %v", err)
	}
	close(results)
	var got []Result
	for r := range results {
		got = append(got, r)
	}
	if want := []Result{Listening, Match}; !slices.Equal(got, want) {
		t.Errorf("results %v, want %v", got, want)
	}
}
