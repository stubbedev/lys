package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/splash/internal/client"
	"github.com/stubbedev/lys/splash/internal/daemon"
	"github.com/stubbedev/lys/splash/internal/protocol"
	"github.com/stubbedev/lys/splash/internal/server"
)

func TestRoleOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		argv []string
		want role
	}{
		{[]string{"/usr/sbin/plymouthd", "--mode=boot"}, plymouthdRole},
		{[]string{"plymouthd"}, plymouthdRole},
		{[]string{"@/usr/sbin/plymouthd", "--mode=boot"}, plymouthdRole},
		{[]string{"/usr/bin/plymouth", "--ping"}, clientRole},
		{[]string{"plymouth", "--mode=boot"}, clientRole},
		{[]string{"/bin/lys-splash", "--mode=boot"}, daemonRole},
		{[]string{"@/bin/lys-splash", "--mode", "shutdown"}, daemonRole},
		{[]string{"lys-splash", "quit", "--retain-splash"}, clientRole},
		{[]string{"lys-splash", "--ping"}, clientRole},
		{[]string{"lys-splash"}, clientRole},
		{[]string{"/usr/lib/systemd/system-shutdown/lys-splash-shutdown-hook", "reboot"}, shutdownHookRole},
		{[]string{"/nix/store/kqg81gwxr25s9ggxqi0m6hqiz9k6lj8w-system-shutdown/lys-splash-shutdown-hook", "reboot"}, shutdownHookRole},
		{[]string{"lys-splash-shutdown-hook", "poweroff"}, shutdownHookRole},
		{[]string{"/usr/lib/systemd/system-shutdown/lys-splash", "reboot"}, clientRole},
		{[]string{"/usr/lib/systemd/system-shutdown/plymouth", "kexec"}, clientRole},
		{[]string{"/usr/bin/lys-splash", "reboot"}, clientRole},
	}
	for _, tc := range tests {
		if got := roleOf(tc.argv); got != tc.want {
			t.Errorf("roleOf(%q) = %d, want %d", tc.argv, got, tc.want)
		}
	}
}

func TestParseOptions(t *testing.T) {
	t.Parallel()
	o, err := parseOptions("plymouthd", []string{"--mode=reboot", "--attach-to-session", "--pid-file=/run/plymouth/pid"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if o.mode != daemon.Reboot || o.pidFile != "/run/plymouth/pid" || o.theme != defaultTheme || o.noDaemon {
		t.Errorf("parseOptions = %+v", o)
	}
	if o, err := parseOptions("lys-splash", nil, io.Discard); err != nil || o.mode != daemon.Boot {
		t.Errorf("the default mode = %+v, %v", o, err)
	}
	for _, args := range [][]string{{"--mode=suspend"}, {"--frobnicate"}, {"--mode=boot", "extra"}} {
		if _, err := parseOptions("lys-splash", args, io.Discard); err == nil {
			t.Errorf("parseOptions(%q) succeeded", args)
		}
	}
}

func TestDetachedStderrPrefersTheKernelsLog(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logged, nil))
	device := filepath.Join(t.TempDir(), "kmsg")
	if err := os.WriteFile(device, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, target, closeFile := detachedStderr(device, log)
	if target != logsToKmsg || f.Name() != device {
		t.Errorf("with the device present, stderr = %s and target = %s", f.Name(), target)
	}
	if err := closeFile(); err != nil {
		t.Error(err)
	}
	if logged.Len() != 0 {
		t.Errorf("logged with the device present: %s", logged.String())
	}

	f, target, closeFile = detachedStderr(device+"-missing", log)
	if target != logsToStderr || f != os.Stderr {
		t.Errorf("with no device, stderr = %s and target = %s", f.Name(), target)
	}
	if err := closeFile(); err != nil {
		t.Error(err)
	}
	if !strings.Contains(logged.String(), "kmsg-missing") {
		t.Errorf("the missing device was not reported: %q", logged.String())
	}
}

func TestListenWithADaemonRunningReportsReady(t *testing.T) {
	t.Parallel()
	quiet := slog.New(slog.DiscardHandler)
	socket := "@lys-splash-main-test-" + rand.Text()
	addr := "@lys-splash-main-notify-" + rand.Text()
	ready, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Close()
	first, err := listen(socket, addr, quiet)
	if err != nil || first == nil {
		t.Fatalf("the first daemon's listen = %v, %v", first, err)
	}
	defer first.Close()
	second, err := listen(socket, addr, quiet)
	if err != nil || second != nil {
		t.Fatalf("the second daemon's listen = %v, %v; want no server and no error", second, err)
	}
	if err := ready.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := ready.Read(buf)
	if err != nil || string(buf[:n]) != "READY=1" {
		t.Errorf("the second daemon notified %q, %v; want READY=1", buf[:n], err)
	}
}

const deadStderrHelper = "LYS_SPLASH_TEST_DEAD_STDERR"

func TestTheDaemonOutlivesItsStandardErrorsReader(t *testing.T) {
	t.Parallel()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.NewFile(uintptr(fds[0]), "stderr")
	defer stderr.Close()
	if err := syscall.Close(fds[1]); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDeadStderrHelper$")
	cmd.Env = append(os.Environ(), deadStderrHelper+"=1")
	cmd.Stderr = stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the daemon's run = %v, want its exit status for the bad option", err)
	}
	if code := exit.ExitCode(); code != 1 {
		t.Errorf("the daemon ended with %v, want exit status 1", exit)
	}
}

func TestDeadStderrHelper(t *testing.T) {
	if os.Getenv(deadStderrHelper) == "" {
		t.Skip("run by TestTheDaemonOutlivesItsStandardErrorsReader")
	}
	os.Exit(runDaemon([]string{"lys-splash", "--mode=bogus"}, false))
}

type recordingHandler struct {
	requests chan protocol.Request
}

func (h recordingHandler) Request(c *server.Call) {
	h.requests <- c.Request
	_ = c.Reply(protocol.Ack{})
}

func (recordingHandler) HangUp(*server.Conn) {}

func hookLog() (*slog.Logger, *bytes.Buffer) {
	var logs bytes.Buffer
	return slog.New(slog.NewTextHandler(&logs, nil)), &logs
}

func listenFor(t *testing.T, owner uint32) (string, chan protocol.Request) {
	t.Helper()
	socket := "@lys-splash-main-hook-" + rand.Text()
	srv, err := server.Listen(socket, owner, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	h := recordingHandler{requests: make(chan protocol.Request, 4)}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(h) }()
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Error(err)
		}
		<-served
	})
	return socket, h.requests
}

const hookPath = "/nix/store/kqg81gwxr25s9ggxqi0m6hqiz9k6lj8w-system-shutdown/" + shutdownHook

func TestTheShutdownHookAsksTheSplashToRetainItsFrame(t *testing.T) {
	t.Parallel()
	socket, requests := listenFor(t, uint32(os.Getuid()))
	c := client.Client{Socket: socket, Stdout: io.Discard, Stderr: io.Discard}
	for _, args := range [][]string{{"reboot"}, {"poweroff"}, {""}, {}} {
		log, logs := hookLog()
		if got := runShutdownHook(c, append([]string{hookPath}, args...), log); got != client.ExitOK {
			t.Fatalf("the hook with %q exited %d: %s", args, got, logs.String())
		}
		if got := <-requests; got != (protocol.Quit{Retain: true}) {
			t.Errorf("the hook with %q sent %#v, want a quit retaining the splash", args, got)
		}
		for _, want := range []string{
			fmt.Sprintf(`level=INFO msg="asking the splash to fade its status group out" args=%v`, args),
			fmt.Sprintf(`level=INFO msg="the splash faded its status group out" args=%v`, args),
		} {
			if !strings.Contains(logs.String(), want) {
				t.Errorf("the hook with %q did not log %s; logged:\n%s", args, want, logs.String())
			}
		}
	}
}

func TestTheShutdownHookLogsASplashRefusingIt(t *testing.T) {
	t.Parallel()
	socket, requests := listenFor(t, uint32(os.Getuid())+1)
	log, logs := hookLog()
	c := client.Client{Socket: socket, Stdout: io.Discard, Stderr: io.Discard}
	if got := runShutdownHook(c, []string{hookPath, "reboot"}, log); got != client.ExitFailed {
		t.Errorf("the hook refused by the splash exited %d, want %d", got, client.ExitFailed)
	}
	want := `level=ERROR msg="asking the splash to fade its status group out" args=[reboot] exit=1`
	if !strings.Contains(logs.String(), want) {
		t.Errorf("the hook refused by the splash did not log %s; logged:\n%s", want, logs.String())
	}
	select {
	case r := <-requests:
		t.Errorf("a refused request reached the splash: %#v", r)
	default:
	}
}

func TestTheShutdownHookWithoutASplashHasNothingToFade(t *testing.T) {
	t.Parallel()
	log, logs := hookLog()
	c := client.Client{Socket: "@lys-splash-main-hook-absent-" + rand.Text(), Stdout: io.Discard, Stderr: io.Discard}
	if got := runShutdownHook(c, []string{hookPath, "reboot"}, log); got != client.ExitOK {
		t.Errorf("the hook with no splash running exited %d, want 0", got)
	}
	want := `level=INFO msg="no splash runs, so there is no status group to fade out" args=[reboot]`
	if !strings.Contains(logs.String(), want) || strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("the hook with no splash running logged %q, want %s and no error", logs.String(), want)
	}
}

const memoryLockHelper = "LYS_SPLASH_TEST_LOCK_MODE"

func mayLockEveryPage(t *testing.T) bool {
	t.Helper()
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &limit); err != nil {
		t.Fatalf("reading RLIMIT_MEMLOCK: %v", err)
	}
	if limit.Cur == unix.RLIM_INFINITY {
		return true
	}
	var caps [2]unix.CapUserData
	if err := unix.Capget(&unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}, &caps[0]); err != nil {
		t.Fatalf("reading this process's capabilities: %v", err)
	}
	return caps[unix.CAP_IPC_LOCK/32].Effective&(1<<(unix.CAP_IPC_LOCK%32)) != 0
}

func lockedKiB(t *testing.T, mode daemon.Mode) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMemoryLockHelper$")
	cmd.Env = append(os.Environ(), memoryLockHelper+"="+mode.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the %s helper: %v; stderr:\n%s", mode, err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Errorf("the %s helper logged:\n%s", mode, stderr.String())
	}
	kib, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatalf("the %s helper printed %q, not VmLck in kB: %v", mode, out, err)
	}
	return kib
}

func TestOnlyTheStoppingModesLockTheirMemory(t *testing.T) {
	t.Parallel()
	for _, mode := range []daemon.Mode{daemon.Boot, daemon.Updates} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			if kib := lockedKiB(t, mode); kib != 0 {
				t.Errorf("the splash in %s mode locked %d kB, want none", mode, kib)
			}
		})
	}
	for _, mode := range []daemon.Mode{daemon.Shutdown, daemon.Reboot, daemon.Kexec} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			if !mayLockEveryPage(t) {
				t.Skip("locking every page of a Go process takes CAP_IPC_LOCK or an unlimited RLIMIT_MEMLOCK; checks.vms-luks runs this as root")
			}
			if kib := lockedKiB(t, mode); kib == 0 {
				t.Errorf("the splash in %s mode locked no memory", mode)
			}
		})
	}
}

func TestMemoryLockHelper(t *testing.T) {
	name := os.Getenv(memoryLockHelper)
	if name == "" {
		t.Skip("run by TestOnlyTheStoppingModesLockTheirMemory")
	}
	mode, err := daemon.ParseMode(name)
	if err != nil {
		t.Fatal(err)
	}
	lockMemoryFor(mode, unix.Mlockall, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(status), "\n") {
		if value, ok := strings.CutPrefix(line, "VmLck:"); ok {
			fmt.Println(strings.TrimSuffix(strings.TrimSpace(value), " kB"))
			os.Exit(0)
		}
	}
	t.Fatalf("no VmLck in /proc/self/status:\n%s", status)
}

func TestTheModesThatOutliveTheFilesystemsAskToLockEveryPage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode daemon.Mode
		want []int
	}{
		{daemon.Boot, nil},
		{daemon.Updates, nil},
		{daemon.Shutdown, []int{unix.MCL_CURRENT | unix.MCL_FUTURE}},
		{daemon.Reboot, []int{unix.MCL_CURRENT | unix.MCL_FUTURE}},
		{daemon.Kexec, []int{unix.MCL_CURRENT | unix.MCL_FUTURE}},
	}
	for _, tc := range tests {
		var asked []int
		logs, buf := hookLog()
		lockMemoryFor(tc.mode, func(flags int) error {
			asked = append(asked, flags)
			return nil
		}, logs)
		if !slices.Equal(asked, tc.want) {
			t.Errorf("%s mode asked mlockall for %#x, want %#x", tc.mode, asked, tc.want)
		}
		if buf.Len() > 0 {
			t.Errorf("%s mode logged a lock that succeeded:\n%s", tc.mode, buf.String())
		}
	}
}

func TestARefusedMemoryLockIsLoggedAsAnError(t *testing.T) {
	t.Parallel()
	logs, buf := hookLog()
	lockMemoryFor(daemon.Reboot, func(int) error { return unix.EPERM }, logs)
	got := buf.String()
	for _, want := range []string{"level=ERROR", "mode=reboot", "mlockall(MCL_CURRENT|MCL_FUTURE)", unix.EPERM.Error()} {
		if !strings.Contains(got, want) {
			t.Errorf("the refused lock logged %q, missing %q", got, want)
		}
	}
}
