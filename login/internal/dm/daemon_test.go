package dm

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/xkb"
	"go.yaml.in/yaml/v3"

	"github.com/stubbedev/lys/login/internal/auth"
	"github.com/stubbedev/lys/login/internal/fprint"
	"github.com/stubbedev/lys/login/internal/greeter"
	"github.com/stubbedev/lys/login/internal/keyboard"
	"github.com/stubbedev/lys/login/internal/keyboard/keyboardtest"
)

const quietWindow = 100 * time.Millisecond

// TestMain lets the test binary be a session worker whose PAM is a
// script: the password "pw" authenticates, anything else fails.
func TestMain(m *testing.M) {
	if code, ran := runFakeMode(); ran {
		os.Exit(code)
	}
	if len(os.Args) > 2 && os.Args[1] == "worker" && os.Args[2] == "session" {
		os.Exit(fakeSessionWorker())
	}
	testhome.Run(m)
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func fakeSessionWorker() int {
	ch := newChannel(os.NewFile(3, "dm"))
	var req request
	if err := ch.recv(&req); err != nil || req.Op != opAuth {
		return 1
	}
	if slices.Contains(os.Args, "--autologin") {
		if !slices.Contains(os.Args, AutologinService) || slices.Contains(os.Args, "--fingerprint-service") {
			_ = ch.send(event{Kind: evFailed, Text: "not the autologin service"})
			return 0
		}
		_ = ch.send(event{Kind: evAuthenticated})
		if err := ch.recv(&req); err != nil || req.Op != opStart {
			return 0
		}
		_ = ch.send(event{Kind: evStarted, UID: 1000, SessionID: "7"})
		_ = ch.send(event{Kind: evEnded, Text: "exited"})
		return 0
	}
	fingerprint := false
	if i := slices.Index(os.Args, "--fingerprint-service"); i >= 0 {
		if i+1 >= len(os.Args) || os.Args[i+1] != FingerprintService {
			_ = ch.send(event{Kind: evFailed, Text: "not the fingerprint service"})
			return 0
		}
		fingerprint = true
	}
	if record := os.Getenv(fakeSessionRecordEnv); record != "" {
		defer func() { appendTo(record, req.Op) }()
	}
	secret := &auth.Prompt{Kind: auth.PromptSecret, Text: "Password:"}
	switch {
	case req.User == "finger" && fingerprint:
		_ = ch.send(event{Kind: evFingerprint, Fingerprint: fprint.Listening})
		_ = ch.send(event{Kind: evPrompt, Prompt: secret})
		_ = ch.send(event{Kind: evAuthenticated})
		for {
			if err := ch.recv(&req); err != nil || req.Op != opAnswer {
				break
			}
		}
		if req.Op != opStart {
			return 0
		}
		_ = ch.send(event{Kind: evStarted, UID: 1001})
		_ = ch.send(event{Kind: evEnded, Text: "exited"})
		return 0
	case req.User == "nomatch" && fingerprint:
		_ = ch.send(event{Kind: evPrompt, Prompt: secret})
		_ = ch.send(event{Kind: evFingerprint, Fingerprint: fprint.NoMatch})
	default:
		_ = ch.send(event{Kind: evPrompt, Prompt: secret})
	}
	if err := ch.recv(&req); err != nil || req.Op != opAnswer || req.Answer == nil || *req.Answer != "pw" {
		_ = ch.send(event{Kind: evFailed, Text: "Authentication failure"})
		return 0
	}
	_ = ch.send(event{Kind: evAuthenticated})
	if err := ch.recv(&req); err != nil || req.Op != opStart {
		return 0
	}
	_ = ch.send(event{Kind: evStarted, UID: 1000})
	_ = ch.send(event{Kind: evEnded, Text: "exited"})
	return 0
}

const fakeSessionRecordEnv = "LYS_LOGIN_FAKE_SESSION_RECORD"

func appendTo(path, line string) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		panic(err)
	}
}

func testRequests() greeterRequests {
	return greeterRequests{vt: 7, method: passwordOrFingerprintLogin, existing: func(string) bool { return false }}
}

func TestAWorkerThatCannotBeReapedHasItsHandleReleased(t *testing.T) {
	before := openDescriptors()
	self, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	err = (&worker{proc: self, ch: newChannel(r), f: r}).wait()
	if !errors.Is(err, syscall.ECHILD) {
		t.Errorf("waiting for a process that is not a child: %v, want ECHILD", err)
	}
	if after := openDescriptors(); after != before {
		t.Errorf("%d descriptors open after the wait, %d before", after, before)
	}
	runtime.KeepAlive(self)
}

func finishLogin(t *testing.T, login *pendingLogin) []uint32 {
	t.Helper()
	defer func() {
		if err := login.w.wait(); err != nil {
			t.Errorf("reap the worker: %v", err)
		}
	}()
	if err := login.w.ch.send(request{Op: opStart, Cmd: login.cmd, Env: login.env}); err != nil {
		t.Fatal(err)
	}
	var started []uint32
	for {
		var ev event
		if err := login.w.ch.recv(&ev); err != nil {
			t.Fatalf("the session's events: %v", err)
		}
		switch ev.Kind {
		case evStarted:
			started = append(started, ev.UID)
		case evEnded:
			return started
		}
	}
}

func testOptions() Options {
	return Options{Service: "lys-login", VT: 7, Self: os.Args[0]}
}

// serve runs the daemon's side of one greeter connection.
func serve(t *testing.T) (net.Conn, <-chan *pendingLogin, <-chan error) {
	t.Helper()
	return serveWith(t, testRequests())
}

func serveWith(t *testing.T, r greeterRequests) (net.Conn, <-chan *pendingLogin, <-chan error) {
	t.Helper()
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	logins, errs := make(chan *pendingLogin, 1), make(chan error, 1)
	go func() {
		l, err := testOptions().serveGreeter(server, r)
		logins <- l
		errs <- err
	}()
	return client, logins, errs
}

// The greeter's greetd client logs in through the daemon: the worker's
// prompt reaches it, its answer reaches the worker, and start_session
// hands the daemon the session to start, which then runs.
func TestGreeterLogsInThroughTheDaemon(t *testing.T) {
	client, logins, errs := serve(t)
	g := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}, Env: []string{"LANG=C"}} }})
	var prompts []auth.Prompt
	err := g.Run("alice", func(p auth.Prompt) (string, bool) {
		prompts = append(prompts, p)
		return "pw", true
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	login := <-logins
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 1 || prompts[0].Text != "Password:" {
		t.Errorf("prompts = %+v", prompts)
	}
	if !slices.Equal(login.cmd, []string{"Hyprland"}) || !slices.Equal(login.env, []string{"LANG=C"}) {
		t.Errorf("session = %v %v", login.cmd, login.env)
	}
	started := finishLogin(t, login)
	if !slices.Equal(started, []uint32{1000}) {
		t.Errorf("started = %v, want the session's uid once", started)
	}
}

// A wrong password is an authentication error to the greeter. The
// greeter retries on a connection of its own, which the daemon serves
// while the greeter runs, and that login goes through.
func TestAFailedLoginCanBeRetried(t *testing.T) {
	conns := make(chan net.Conn)
	exited := make(chan struct{})
	logins := make(chan *pendingLogin, 1)
	go func() {
		g, _ := testOptions().serveConns(context.Background(), conns, exited, testRequests())
		logins <- g.login
	}()
	attempt := func(password string) error {
		server, client := net.Pipe()
		t.Cleanup(func() { _ = client.Close() })
		conns <- server
		err := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}} }}).Run("alice", func(auth.Prompt) (string, bool) { return password, true })
		if err != nil {
			_ = client.Close()
		}
		return err
	}
	if err := attempt("wrong"); err == nil || !strings.Contains(err.Error(), "Authentication failure") {
		t.Fatalf("wrong password = %v", err)
	}
	if err := attempt("pw"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	login := <-logins
	if login == nil {
		t.Fatal("no session after the retry")
	}
	finishLogin(t, login)
}

// The greeter going away ends the serving without a session.
func TestServeConnsEndsWithTheGreeter(t *testing.T) {
	exited := make(chan struct{})
	close(exited)
	if g, err := testOptions().serveConns(context.Background(), make(chan net.Conn), exited, testRequests()); g != (greeted{}) || err != nil {
		t.Errorf("serveConns = %v, %v; want neither a login nor an error", g, err)
	}
}

// Requests out of turn are errors, never a session.
func TestRequestsOutOfTurnAreRefused(t *testing.T) {
	client, _, errs := serve(t)
	read := func() string {
		t.Helper()
		var resp map[string]any
		if err := readFrame(client, &resp); err != nil {
			t.Fatal(err)
		}
		return resp["type"].(string) + ":" + strings.TrimSpace(asString(resp["error_type"]))
	}
	for _, req := range []map[string]any{
		{"type": auth.GreetdStartSession, "cmd": []string{"sh"}, "env": []string{}},
		{"type": auth.GreetdPostResponse, "response": "pw"},
	} {
		if err := writeFrame(client, req); err != nil {
			t.Fatal(err)
		}
		if got := read(); got != "error:error" {
			t.Errorf("%v = %s, want an error", req["type"], got)
		}
	}
	if err := writeFrame(client, map[string]any{"type": auth.GreetdCancelSession}); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "success:" {
		t.Errorf("cancel without a login = %s", got)
	}
	_ = client.Close()
	if err := <-errs; err != nil {
		t.Errorf("a greeter closing between requests = %v, want a clean end", err)
	}
}

// A greeter that hangs up inside a request is an error, unlike one that
// hangs up between requests.
func TestATruncatedRequestIsAnError(t *testing.T) {
	client, logins, errs := serve(t)
	if _, err := client.Write([]byte{0x10, 0}); err != nil {
		t.Fatal(err)
	}
	_ = client.Close()
	if l, err := <-logins, <-errs; l != nil || err == nil {
		t.Errorf("serveGreeter = %v, %v; want an error and no login", l, err)
	}
}

func TestKioskConfigQuotesTheGreeter(t *testing.T) {
	got := kioskConfig(kiosk{greet: []string{"/nix/store/x/bin/lys-login", "greet", "--sessions", "/run/my sessions", `--env`, `A="b"`}, cursor: greeter.Cursor{Size: 24}})
	want := "binds: []\n" + kioskLook + "env:\n  XCURSOR_SIZE: \"24\"\nautostart:\n  - \"/nix/store/x/bin/lys-login greet --sessions '/run/my sessions' --env 'A=\\\"b\\\"'\"\n"
	if got != want {
		t.Errorf("kiosk config =\n%s\nwant\n%s", got, want)
	}
}

func TestKioskConfigDrawsNoBorder(t *testing.T) {
	var cfg struct {
		Borders struct {
			Width *int `yaml:"width"`
		} `yaml:"borders"`
	}
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}})), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Borders.Width == nil || *cfg.Borders.Width != 0 {
		t.Errorf("borders.width = %v, want 0 so lys-wm draws no border around the greeter", cfg.Borders.Width)
	}
}

// A kiosk config leaves out nothing lys-wm's default look would draw around
// the greeter or animate on the way to it.
func TestKioskConfigTurnsOffTheDefaultLook(t *testing.T) {
	var cfg struct {
		Gaps struct {
			Inner *int `yaml:"inner"`
			Outer *int `yaml:"outer"`
		} `yaml:"gaps"`
		Decoration struct {
			Rounding *int `yaml:"rounding"`
			Shadow   struct {
				Enabled *bool `yaml:"enabled"`
			} `yaml:"shadow"`
		} `yaml:"decoration"`
		Animations struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"animations"`
	}
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}})), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Gaps.Inner == nil || *cfg.Gaps.Inner != 0 || cfg.Gaps.Outer == nil || *cfg.Gaps.Outer != 0 {
		t.Errorf("gaps = %+v, want 0 so the greeter reaches the edges", cfg.Gaps)
	}
	if cfg.Decoration.Rounding == nil || *cfg.Decoration.Rounding != 0 {
		t.Errorf("decoration.rounding = %v, want 0 so no corner shows lys-wm's background", cfg.Decoration.Rounding)
	}
	if cfg.Decoration.Shadow.Enabled == nil || *cfg.Decoration.Shadow.Enabled {
		t.Errorf("decoration.shadow.enabled = %v, want false", cfg.Decoration.Shadow.Enabled)
	}
	if cfg.Animations.Enabled == nil || *cfg.Animations.Enabled {
		t.Errorf("animations.enabled = %v, want false so the greeter does not fade in over lys-wm's background", cfg.Animations.Enabled)
	}
}

// lys-wm draws the greeter's cursor, from the theme and size in the
// kiosk config's environment, which the greeter inherits.
func TestKioskConfigCarriesTheCursor(t *testing.T) {
	var cfg struct {
		Env map[string]string `yaml:"env"`
	}
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Theme: `Bibata "Ice"`, Size: 48}})), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"XCURSOR_THEME": `Bibata "Ice"`, "XCURSOR_SIZE": "48"}; !reflect.DeepEqual(cfg.Env, want) {
		t.Errorf("env = %v, want %v", cfg.Env, want)
	}
	cfg.Env = nil
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}})), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"XCURSOR_SIZE": "24"}; !reflect.DeepEqual(cfg.Env, want) {
		t.Errorf("no theme: env = %v, want %v, lys-wm's own theme", cfg.Env, want)
	}
}

func TestShellOf(t *testing.T) {
	passwd := "root:x:0:0::/root:/run/current-system/sw/bin/bash\nalice:x:1000:100::/home/alice:/bin/zsh\nnoshell:x:1001:100::/home/n:\n"
	for name, want := range map[string]string{"alice": "/bin/zsh", "noshell": "/bin/sh", "ghost": "/bin/sh"} {
		if got := shellOf(passwd, name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// writeFrame and readFrame speak greetd's framing as a raw greeter.
func writeFrame(w io.Writer, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(binary.NativeEndian.AppendUint32(nil, uint32(len(body))), body...))
	return err
}

func readFrame(r io.Reader, v any) error {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return err
	}
	body := make([]byte, binary.NativeEndian.Uint32(head[:]))
	if _, err := io.ReadFull(r, body); err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}

// A connection the greeter left open does not hold up the next: the
// daemon serves each on its own.
func TestAnIdleConnectionDoesNotBlockTheNext(t *testing.T) {
	conns := make(chan net.Conn)
	logins := make(chan *pendingLogin, 1)
	go func() {
		g, _ := testOptions().serveConns(context.Background(), conns, make(chan struct{}), testRequests())
		logins <- g.login
	}()
	idleServer, idleClient := net.Pipe()
	t.Cleanup(func() { _ = idleClient.Close() })
	conns <- idleServer
	server, client := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	conns <- server
	if err := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}} }}).Run("alice", func(auth.Prompt) (string, bool) { return "pw", true }); err != nil {
		t.Fatalf("login beside an idle connection: %v", err)
	}
	login := <-logins
	if login == nil {
		t.Fatal("no session")
	}
	finishLogin(t, login)
	if _, err := idleClient.Write([]byte{0}); err == nil {
		t.Error("the idle connection stayed open after the login")
	}
}

// A keyring thread that cannot take the session's user's credentials
// hands nothing over, says why, and leaves the worker as it was.
func TestUnlockKeyringAsRefusedCredentials(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root may take any credentials")
	}
	other := account{name: "other", uid: uint32(os.Getuid()) + 1, gid: uint32(os.Getgid()) + 1}
	password := []byte("pw")
	err := <-unlockKeyringAs(other, []string{"XDG_RUNTIME_DIR=" + t.TempDir()}, password, nil)
	if err == nil || !strings.Contains(err.Error(), "other") {
		t.Errorf("unlock as another user = %v, want the credentials refused", err)
	}
	if os.Getuid() == int(other.uid) || os.Getgid() == int(other.gid) {
		t.Error("the worker took the keyring thread's credentials")
	}
	if string(password) == "pw" {
		t.Error("the password outlived the hand-over")
	}
}

func TestLookupIn(t *testing.T) {
	env := lookupIn([]string{"A=1", "B=x=y", "A=2", "C"})
	for name, want := range map[string]string{"A": "2", "B": "x=y", "C": "", "D": ""} {
		if got := env(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestGreeterKnowsTheRuntimeDir(t *testing.T) {
	o := testOptions()
	o.GreeterUser, o.RuntimeDir, o.Compositor = "lys-greeter", "/run/lys-login", []string{"lys-wm"}
	got := o.greeterWorkerArgs("/run/lys-login/greeter.sock", "/run/lys-login/kiosk.yaml", o.VT)
	want := []string{
		"worker", "greeter", "--service", "lys-login", "--vt", "7", "--user", "lys-greeter",
		"--env", auth.GreetdSockEnv + "=/run/lys-login/greeter.sock", "--env", RuntimeDirEnv + "=/run/lys-login", "--",
		"lys-wm", "--config", "/run/lys-login/kiosk.yaml",
	}
	if !slices.Equal(got, want) {
		t.Errorf("greeter worker args = %q, want %q", got, want)
	}
}

func TestTheGreeterHandsOverToTheDisplayManager(t *testing.T) {
	greet, err := greeter.ParseOptions(greeter.SystemDefaults(func(string) string { return "" }), []string{"--", "fallback-session"})
	if err != nil {
		t.Fatal(err)
	}
	o := Options{Self: "/bin/lys-login", Greeter: greet}
	want := []string{
		"/bin/lys-login", "greet", "--config", "/etc/lys-shell/config.yaml", "--state", "/var/lib/lys-greeter/last-session",
		"--sessions", "/usr/local/share/wayland-sessions", "--sessions", "/usr/share/wayland-sessions",
		"--xsessions", "/usr/local/share/xsessions", "--xsessions", "/usr/share/xsessions",
		"--hand-over", "--", "fallback-session",
	}
	if got := o.greeterCommand(); !slices.Equal(got, want) {
		t.Errorf("greeter command %q, want %q", got, want)
	}
}

type kioskKeyboard struct {
	Input struct {
		Keyboard map[string]string `yaml:"keyboard"`
	} `yaml:"input"`
	Env map[string]string `yaml:"env"`
}

func greeterKiosk(t *testing.T, o Options) kioskKeyboard {
	t.Helper()
	var cfg kioskKeyboard
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}, keyboard: o.greeterKeyboard()})), &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func keyboardOptions(t *testing.T, xorg string) Options {
	t.Helper()
	dir := t.TempDir()
	o := Options{Keyboard: keyboard.Sources{Xorg: filepath.Join(dir, "00-keyboard.conf"), Vconsole: filepath.Join(dir, "vconsole.conf")}}
	if xorg != "" {
		if err := os.WriteFile(o.Keyboard.Xorg, []byte(xorg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return o
}

func TestTheKioskTypesWithLocaledsLayout(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.ServeLocale1(t, xkb.Names{Model: "pc105", Layout: "de,us", Variant: "nodeadkeys,", Options: "grp:alt_shift_toggle"})
	cfg := greeterKiosk(t, keyboardOptions(t, `Option "XkbLayout" "fr"`))
	want := map[string]string{"model": "pc105", "layout": "de,us", "variant": "nodeadkeys,", "options": "grp:alt_shift_toggle"}
	if !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
	for name := range cfg.Env {
		if strings.HasPrefix(name, "XKB_") {
			t.Errorf("the kiosk sets %s: lys-wm takes the layout from input.keyboard", name)
		}
	}
}

func TestWithoutLocaledTheKioskTypesWithTheXorgFilesLayout(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.NoSystemBus(t)
	cfg := greeterKiosk(t, keyboardOptions(t, "Section \"InputClass\"\n  Option \"XkbLayout\" \"fr\"\nEndSection\n"))
	if want := map[string]string{"layout": "fr"}; !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
}

func TestALayoutLysWMCannotCompileLeavesTheKioskOnItsDefault(t *testing.T) {
	keyboardtest.XKBData(t)
	keyboardtest.ServeLocale1(t, xkb.Names{Layout: "no-such-layout"})
	var logged strings.Builder
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	cfg := greeterKiosk(t, keyboardOptions(t, ""))
	if cfg.Input.Keyboard != nil {
		t.Errorf("input.keyboard = %v, want none, so lys-wm still starts", cfg.Input.Keyboard)
	}
	if !strings.Contains(logged.String(), "no-such-layout") {
		t.Errorf("the refused layout was not logged: %q", logged.String())
	}
}

func TestNoSystemLayoutLeavesInputOut(t *testing.T) {
	if got := kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}}); strings.Contains(got, "input:") {
		t.Errorf("a kiosk without a system layout names one:\n%s", got)
	}
}

func TestTheRememberedStickyAndSlowKeysReachTheKiosk(t *testing.T) {
	state := filepath.Join(t.TempDir(), "last-session")
	if err := greeter.SaveLast(greeter.AccessibilityPath(state), "sticky-keys\nslow-keys\nlarge-text\n"); err != nil {
		t.Fatal(err)
	}
	o := Options{Greeter: greeter.Options{StatePath: state}}
	var cfg kioskKeyboard
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}, keyboard: xkb.Names{Layout: "de"}, a11y: o.greeterAccessibility()})), &cfg); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"layout": "de", "sticky_keys": "true", "slow_keys": "300"}
	if !reflect.DeepEqual(cfg.Input.Keyboard, want) {
		t.Errorf("input.keyboard = %v, want %v", cfg.Input.Keyboard, want)
	}
	var bare kioskKeyboard
	if err := yaml.Unmarshal([]byte(kioskConfig(kiosk{greet: []string{"lys-login", "greet"}, cursor: greeter.Cursor{Size: 24}, a11y: greeter.Accessibility{greeter.StickyKeys: true}})), &bare); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"sticky_keys": "true"}; !reflect.DeepEqual(bare.Input.Keyboard, want) {
		t.Errorf("without a layout input.keyboard = %v, want %v", bare.Input.Keyboard, want)
	}
}

func TestAFingerprintLogsInWhileThePasswordIsAskedFor(t *testing.T) {
	client, logins, errs := serve(t)
	unanswered := make(chan struct{})
	defer close(unanswered)
	var fingerprints []fprint.Result
	g := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}} }})
	err := g.Run("finger", func(p auth.Prompt) (string, bool) {
		if p.Kind == auth.PromptFingerprint {
			fingerprints = append(fingerprints, p.Fingerprint)
		}
		if p.WantsInput() {
			<-unanswered
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("a fingerprint login with the password unanswered: %v", err)
	}
	login := <-logins
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fingerprints, []fprint.Result{fprint.Listening}) {
		t.Errorf("fingerprint statuses %v, want the reader listening", fingerprints)
	}
	if started := finishLogin(t, login); !slices.Equal(started, []uint32{1001}) {
		t.Errorf("started = %v, want the fingerprint's session", started)
	}
}

func TestAnAnswerAfterAFingerprintLoginGetsNoReply(t *testing.T) {
	client, logins, errs := serve(t)
	read := func() map[string]any {
		t.Helper()
		var resp map[string]any
		if err := readFrame(client, &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}
	if err := writeFrame(client, map[string]any{"type": auth.GreetdCreateSession, "username": "finger"}); err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, want := range []string{"fingerprint", "auth_message", "success"} {
		resp := read()
		types = append(types, asString(resp["type"]))
		if resp["type"] != want {
			t.Fatalf("responses %v, want the reader, the password prompt, then the fingerprint's success", types)
		}
	}
	if err := writeFrame(client, map[string]any{"type": auth.GreetdPostResponse, "response": "typed too late"}); err != nil {
		t.Fatal(err)
	}
	if err := writeFrame(client, map[string]any{"type": auth.GreetdStartSession, "cmd": []string{"Hyprland"}, "env": []string{}}); err != nil {
		t.Fatal(err)
	}
	if resp := read(); resp["type"] != "success" {
		t.Fatalf("start_session after the late answer = %v, want success: the answer the success already took has no reply of its own", resp)
	}
	login := <-logins
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	finishLogin(t, login)
}

func TestAFingerprintThatDoesNotMatchLeavesThePasswordToLogIn(t *testing.T) {
	for _, c := range []struct {
		password string
		ok       bool
	}{{"pw", true}, {"wrong", false}} {
		client, logins, errs := serve(t)
		var fingerprints []fprint.Result
		g := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}} }})
		err := g.Run("nomatch", func(p auth.Prompt) (string, bool) {
			if p.Kind == auth.PromptFingerprint {
				fingerprints = append(fingerprints, p.Fingerprint)
			}
			if p.WantsInput() {
				return c.password, true
			}
			return "", false
		})
		if !slices.Equal(fingerprints, []fprint.Result{fprint.NoMatch}) {
			t.Errorf("%s: fingerprint statuses %v, want the no-match", c.password, fingerprints)
		}
		if !c.ok {
			if err == nil || !strings.Contains(err.Error(), "Authentication failure") {
				t.Errorf("a wrong password after a no-match = %v, want the refusal", err)
			}
			_ = client.Close()
			if l := <-logins; l != nil {
				t.Error("a wrong password after a no-match handed over a session")
			}
			<-errs
			continue
		}
		if err != nil {
			t.Fatalf("the password after a no-match: %v", err)
		}
		login := <-logins
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		finishLogin(t, login)
	}
}

func recordSession(t *testing.T) func() []string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session")
	t.Setenv(fakeSessionRecordEnv, path)
	return func() []string {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Fields(string(data))
	}
}

func authenticateOver(t *testing.T, client net.Conn, user string) {
	t.Helper()
	read := func(want string) {
		t.Helper()
		var resp map[string]any
		if err := readFrame(client, &resp); err != nil {
			t.Fatal(err)
		}
		if resp["type"] != want {
			t.Fatalf("response %v, want %s", resp, want)
		}
	}
	if err := writeFrame(client, map[string]any{"type": auth.GreetdCreateSession, "username": user}); err != nil {
		t.Fatal(err)
	}
	read("auth_message")
	if err := writeFrame(client, map[string]any{"type": auth.GreetdPostResponse, "response": "pw"}); err != nil {
		t.Fatal(err)
	}
	read("success")
}

func TestAGreeterThatGoesAwayAfterAuthenticatingEndsTheLogin(t *testing.T) {
	ended := recordSession(t)
	client, logins, errs := serve(t)
	authenticateOver(t, client, "alice")
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if login := <-logins; login != nil {
		t.Fatalf("a greeter that went away handed over %+v", login)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if got := ended(); !slices.Equal(got, []string{opCancel}) {
		t.Errorf("the worker's last request %q, want it cancelled so it ends its PAM transaction", got)
	}
}

func TestAGreeterThatStartsTheSessionKeepsTheLogin(t *testing.T) {
	ended := recordSession(t)
	client, logins, errs := serve(t)
	authenticateOver(t, client, "alice")
	if err := writeFrame(client, map[string]any{"type": auth.GreetdStartSession, "cmd": []string{"Hyprland"}, "env": []string{}}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := readFrame(client, &resp); err != nil || resp["type"] != "success" {
		t.Fatalf("start_session = %v, %v; want success", resp, err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	login := <-logins
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if login == nil {
		t.Fatal("start_session handed over no login")
	}
	if got := ended(); got != nil {
		t.Fatalf("the worker ended (%q) before its session was started", got)
	}
	if started := finishLogin(t, login); !slices.Equal(started, []uint32{1000}) {
		t.Errorf("started = %v, want the session's uid once", started)
	}
	if got := ended(); !slices.Equal(got, []string{opStart}) {
		t.Errorf("the worker's last request %q, want the start", got)
	}
}

func TestAnAuthenticatedLoginThatLosesToAnotherIsEnded(t *testing.T) {
	ended := recordSession(t)
	conns := make(chan net.Conn)
	results := make(chan greeted, 1)
	go func() {
		g, _ := testOptions().serveConns(context.Background(), conns, make(chan struct{}), testRequests())
		results <- g
	}()
	connect := func() net.Conn {
		server, client := net.Pipe()
		t.Cleanup(func() { _ = client.Close() })
		conns <- server
		return client
	}
	loser, winner := connect(), connect()
	authenticateOver(t, loser, "bob")
	authenticateOver(t, winner, "alice")
	if err := writeFrame(winner, map[string]any{"type": auth.GreetdStartSession, "cmd": []string{"Hyprland"}, "env": []string{}}); err != nil {
		t.Fatal(err)
	}
	var resp map[string]any
	if err := readFrame(winner, &resp); err != nil || resp["type"] != "success" {
		t.Fatalf("start_session = %v, %v; want success", resp, err)
	}
	g := <-results
	if g.login == nil || g.login.user != "alice" {
		t.Fatalf("serveConns handed over %+v, want alice's login", g.login)
	}
	if got := ended(); !slices.Equal(got, []string{opCancel}) {
		t.Errorf("the other authenticated worker's last request %q, want it cancelled", got)
	}
	finishLogin(t, g.login)
}

func TestWithTheFingerprintOffOnlyThePasswordLogsIn(t *testing.T) {
	r := testRequests()
	r.method = passwordLogin
	for _, c := range []struct {
		password string
		ok       bool
	}{{"pw", true}, {"wrong", false}} {
		client, logins, errs := serveWith(t, r)
		var prompts []auth.Prompt
		g := auth.NewGreetd(client, auth.Start{Session: func() auth.Session { return auth.Session{Cmd: []string{"Hyprland"}} }})
		err := g.Run("finger", func(p auth.Prompt) (string, bool) {
			prompts = append(prompts, p)
			if p.WantsInput() {
				return c.password, true
			}
			return "", false
		})
		if len(prompts) != 1 || prompts[0].Kind != auth.PromptSecret {
			t.Errorf("%s: prompts %+v, want the password prompt alone and no fingerprint reader", c.password, prompts)
		}
		if !c.ok {
			if err == nil || !strings.Contains(err.Error(), "Authentication failure") {
				t.Errorf("a wrong password with the fingerprint off = %v, want the refusal", err)
			}
			_ = client.Close()
			if l := <-logins; l != nil {
				t.Error("a wrong password with the fingerprint off handed over a session")
			}
			<-errs
			continue
		}
		if err != nil {
			t.Fatalf("the password with the fingerprint off: %v", err)
		}
		login := <-logins
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		if started := finishLogin(t, login); !slices.Equal(started, []uint32{1000}) {
			t.Errorf("started = %v, want the password's session", started)
		}
	}
}

func TestEachLoginMethodStartsItsWorker(t *testing.T) {
	o := testOptions()
	o.RuntimeDir = "/run/lys-login"
	for _, c := range []struct {
		method loginMethod
		want   []string
	}{
		{passwordLogin, []string{"--service", "lys-login"}},
		{passwordOrFingerprintLogin, []string{"--service", "lys-login", "--fingerprint-service", FingerprintService}},
		{autoLogin, []string{"--service", AutologinService, "--autologin"}},
	} {
		got, err := o.sessionWorkerArgs(7, c.method)
		if err != nil {
			t.Fatal(err)
		}
		want := append([]string{"worker", "session", "--vt", "7", "--runtime-dir", "/run/lys-login"}, c.want...)
		if !slices.Equal(got, want) {
			t.Errorf("method %d: %q, want %q", c.method, got, want)
		}
	}
	if _, err := o.sessionWorkerArgs(7, autoLogin+1); err == nil || !strings.Contains(err.Error(), "unknown login method") {
		t.Errorf("an unknown method = %v, want refused", err)
	}
}

func greeterConfigFile(t *testing.T, yaml string) Options {
	t.Helper()
	o := testOptions()
	o.Greeter.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(o.Greeter.ConfigPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestTheGreetersConfigChoosesTheLoginMethod(t *testing.T) {
	for _, c := range []struct {
		yaml string
		want loginMethod
	}{
		{"login:\n  greeter:\n    show-user-list: false\n", passwordOrFingerprintLogin},
		{"login:\n  greeter:\n    fingerprint: true\n", passwordOrFingerprintLogin},
		{"login:\n  greeter:\n    fingerprint: false\n", passwordLogin},
	} {
		if got := greeterLoginMethod(greeterConfigFile(t, c.yaml).greeterConfig()); got != c.want {
			t.Errorf("%q: method %d, want %d", c.yaml, got, c.want)
		}
	}
}

func TestAnInvalidFingerprintSettingIsAConfigErrorAndKeepsTheDefault(t *testing.T) {
	o := greeterConfigFile(t, "login:\n  greeter:\n    fingerprint: maybe\n")
	var diags []config.Diagnostic
	cfg, err := config.LoadGreeter(o.Greeter.ConfigPath, func(d config.Diagnostic) { diags = append(diags, d) })
	if err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Kind != config.DiagnosticError || !strings.Contains(diags[0].Plain(), "login.greeter.fingerprint") {
		t.Fatalf("diagnostics %+v, want one error naming the fingerprint key", diags)
	}
	if got := greeterLoginMethod(cfg.Login.Greeter); got != passwordOrFingerprintLogin {
		t.Errorf("method %d after an invalid value, want the default", got)
	}
}

func unixPair(t *testing.T) (server, client net.Conn) {
	t.Helper()
	path := "@lys-dm-test-" + rand.Text()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	client, err = net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server, err = ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	return server, client
}

func TestAConnectionTheDaemonClosesEndsWithoutAnError(t *testing.T) {
	server, _ := unixPair(t)
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	l, err := testOptions().serveGreeter(server, testRequests())
	if l != nil || err != nil {
		t.Fatalf("a connection the daemon closed ended with login %v, error %v; want neither", l, err)
	}
}

func TestAMalformedGreeterRequestIsAnError(t *testing.T) {
	server, client := unixPair(t)
	frame := []byte("{x")
	header := make([]byte, 4)
	binary.NativeEndian.PutUint32(header, uint32(len(frame)))
	if _, err := client.Write(append(header, frame...)); err != nil {
		t.Fatal(err)
	}
	l, err := testOptions().serveGreeter(server, testRequests())
	if l != nil || err == nil || !strings.Contains(err.Error(), "read the greeter's request") {
		t.Fatalf("a malformed request ended with login %v, error %v; want the read error", l, err)
	}
}
