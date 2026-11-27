package dm

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/userdb"
	"github.com/stubbedev/lys/lib/xdg"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/login/internal/auth"
	"github.com/stubbedev/lys/login/internal/fprint"
)

// Seat is the seat the display manager runs on.
const Seat = "seat0"

// sessionPath is the PATH a session starts with when PAM sets none.
const sessionPath = "/run/wrappers/bin:/run/current-system/sw/bin:/usr/local/bin:/usr/bin:/bin"

// profileScript starts a session command the way a login does: the
// system and the user's profile first, then the command in its place.
const profileScript = `[ -f /etc/profile ] && . /etc/profile; [ -f "$HOME/.profile" ] && . "$HOME/.profile"; exec "$@"`

// stopGrace is how long a stopped greeter or session has to exit before it is
// killed.
const stopGrace = 5 * time.Second

// WorkerOptions are what every worker runs with.
type WorkerOptions struct {
	// Service is the PAM service.
	Service string
	// VT is the virtual terminal the sessions run on.
	VT int
	// Self is the lys-login program the workers and the greeter run.
	Self string
	// RuntimeDir holds the lock a session worker keeps while its session runs.
	RuntimeDir         string
	Autologin          bool
	FingerprintService string
}

// greeterSessionType is the type of the session the greeter's compositor
// runs in.
const greeterSessionType = "wayland"

// sessionEnv is the environment pam_systemd registers a session with.
func (o WorkerOptions) sessionEnv(class, sessionType string) []string {
	return []string{
		auth.SessionTypeEnv + "=" + sessionType,
		"XDG_SESSION_CLASS=" + class,
		"XDG_SEAT=" + Seat,
		"XDG_VTNR=" + strconv.Itoa(o.VT),
	}
}

// loginEnv is sessionEnv for a login whose start request carries
// startEnv, which names the type of the session the user picked.
func (o WorkerOptions) loginEnv(startEnv []string) ([]string, error) {
	sessionType := lookupIn(startEnv)(auth.SessionTypeEnv)
	if sessionType == "" {
		return nil, fmt.Errorf("the start request names no %s: %q", auth.SessionTypeEnv, startEnv)
	}
	return o.sessionEnv("user", sessionType), nil
}

// account is who a process runs as.
type account struct {
	name, home, shell string
	uid, gid          uint32
	groups            []uint32
}

var userDB = userdb.SocketPath

// defaultShell is the shell of an account whose record names none.
const defaultShell = "/bin/sh"

// lookupAccount is name's account in systemd's user database: its
// record, then the groups it is a member of besides its primary one.
func lookupAccount(name string) (account, error) {
	ctx := context.Background()
	c, err := userdb.Dial(ctx, userDB)
	if err != nil {
		return account{}, fmt.Errorf("look up %s: %w", name, err)
	}
	defer func() { _ = c.Close() }()
	u, err := c.UserByName(ctx, name)
	if err != nil {
		return account{}, fmt.Errorf("look up %s: %w", name, err)
	}
	if u.Home == "" {
		return account{}, fmt.Errorf("%s's user record names no home directory", name)
	}
	memberships, err := c.Memberships(ctx, name)
	if err != nil {
		return account{}, fmt.Errorf("%s's groups: %w", name, err)
	}
	groups := make([]uint32, 1, 1+len(memberships))
	groups[0] = uint32(u.GID)
	for _, group := range memberships {
		g, err := c.GroupByName(ctx, group)
		if err != nil {
			return account{}, fmt.Errorf("%s's group %s: %w", name, group, err)
		}
		if !slices.Contains(groups, uint32(g.GID)) {
			groups = append(groups, uint32(g.GID))
		}
	}
	return account{name: name, home: u.Home, shell: cmp.Or(u.Shell, defaultShell), uid: uint32(u.UID), gid: uint32(u.GID), groups: groups}, nil
}

// environ is a session's environment: PAM's, then the account's
// basics where PAM set none, then extra.
func (a account) environ(pamEnv, extra []string) []string {
	env := append([]string(nil), pamEnv...)
	set := map[string]bool{}
	for _, kv := range env {
		for i := range len(kv) {
			if kv[i] == '=' {
				set[kv[:i]] = true
				break
			}
		}
	}
	for _, kv := range [][2]string{{"HOME", a.home}, {"USER", a.name}, {"LOGNAME", a.name}, {"SHELL", a.shell}, {"PATH", sessionPath}} {
		if !set[kv[0]] {
			env = append(env, kv[0]+"="+kv[1])
		}
	}
	return append(env, extra...)
}

// activeWait bounds how long logind has to make a session PAM opened
// the active one of its seat.
const activeWait = 10 * time.Second

// openedSession is the logind session pam_systemd registered for the
// session PAM opened with pamEnv.
func openedSession(pamEnv []string) (logind.SessionID, error) {
	id := lookupIn(pamEnv)("XDG_SESSION_ID")
	if id == "" {
		return "", errors.New("PAM opened a session without XDG_SESSION_ID: pam_systemd registered none with logind")
	}
	return logind.SessionID(id), nil
}

// awaitActive returns once logind reports session id active, the one
// in the foreground of its seat that the seat's devices are granted
// to, or fails naming the session and its state when it is not active
// within the bound.
func awaitActive(ctx context.Context, m logind.Manager, id logind.SessionID, within time.Duration) error {
	s, err := m.GetSession(ctx, id)
	if err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, within)
	defer cancel()
	err = s.WaitActive(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		return err
	}
	notActive := fmt.Errorf("logind session %s did not become active within %s", id, within)
	state, stateErr := s.State(ctx)
	if stateErr != nil {
		return errors.Join(notActive, stateErr)
	}
	return fmt.Errorf("%w: it is %s", notActive, state)
}

// startAs starts argv as a once logind has made sessionID, which PAM
// opened for it on the VT the daemon switched to, the active one: the
// seat's devices, which a compositor opens at once, are granted to the
// active session alone. It runs in a session of its own with the VT as
// its controlling terminal and standard input; output goes to the
// worker's log.
func (o WorkerOptions) startAs(sessionID logind.SessionID, a account, argv, env []string) (*exec.Cmd, error) {
	err := withLogind("wait for session "+string(sessionID)+" to become active", func(ctx context.Context, m logind.Manager) error {
		return awaitActive(ctx, m, sessionID, activeWait)
	})
	if err != nil {
		return nil, err
	}
	tty, err := os.OpenFile(fmt.Sprintf("/dev/tty%d", o.VT), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open the VT: %w", err)
	}
	defer func() { _ = tty.Close() }()
	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec // the session the user chose, run as them
	cmd.Env, cmd.Dir = env, a.home
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, os.Stderr, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, Setctty: true, Ctty: 0,
		Credential: &syscall.Credential{Uid: a.uid, Gid: a.gid, Groups: a.groups},
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s as %s: %w", argv[0], a.name, err)
	}
	return cmd, nil
}

// RunGreeter is the greeter's worker: it opens the greeter user's
// session, runs the compositor as that user, and closes the session when
// the compositor exits or the daemon stops the worker (SIGTERM). It
// must run on the process's main thread, as auth.Login says.
func RunGreeter(o WorkerOptions, greeterUser string, compositor, extraEnv []string) error {
	a, err := lookupAccount(greeterUser)
	if err != nil {
		return err
	}
	login, err := auth.Greeter(o.Service, greeterUser)
	if err != nil {
		return err
	}
	defer func() {
		if err := login.Close(); err != nil {
			log.Printf("dm: greeter: %v", err)
		}
	}()
	pamEnv, err := login.OpenSession(o.sessionEnv("greeter", greeterSessionType))
	if err != nil {
		return fmt.Errorf("greeter session: %w", err)
	}
	sessionID, err := openedSession(pamEnv)
	if err != nil {
		return fmt.Errorf("greeter session: %w", err)
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stop)
	cmd, err := o.startAs(sessionID, a, compositor, a.environ(pamEnv, extraEnv))
	if err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		return fmt.Errorf("the greeter's compositor exited: %w", err)
	case <-stop:
		return stopGroup("the greeter", cmd, exited)
	}
}

// stopGroup ends the compositor cmd first, so it never draws a frame
// without what runs in it, then the rest of its process group; SIGKILL
// after the grace. what names it in the log.
func stopGroup(what string, cmd *exec.Cmd, exited <-chan error) error {
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-exited:
		log.Printf("dm: the compositor of %s stopped: %v", what, err)
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		return nil
	case <-time.After(stopGrace):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		err := <-exited
		return fmt.Errorf("%s did not stop in time and was killed: %w", what, err)
	}
}

// keyringWait bounds how long a session's keyring has to start before
// its login password is dropped.
const keyringWait = time.Minute

// unlockKeyringAs hands password to the login keyring of a's session,
// whose environment is env, once one runs there, and reports what the
// keyring said, or why none took it, on the returned channel. The
// session never waits for it. The thread also activates the keyring on
// the session's bus, whose authentication goes by its credentials.
//
// The keyring takes the password from its own user alone, so it is
// handed over from an OS thread of this process that takes a's
// credentials first, as pam_gnome_keyring hands it over from a child of
// gdm's worker that takes them, and ends with the hand-over. Nothing is
// executed for it: under SELinux the session's domain may not enter
// this program, which is labelled as a display manager's. It takes
// password over and clears it before it reports.
func unlockKeyringAs(a account, env []string, password, previous []byte) <-chan error {
	done := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		err := handOverAs(a, env, password, previous)
		clear(password)
		clear(previous)
		done <- err
	}()
	return done
}

func reportKeyring(ch *channel, name string, unlocked <-chan error) {
	err := <-unlocked
	if err == nil {
		return
	}
	text := fmt.Sprintf("dm: the login keyring of %s stays locked: %v", name, err)
	if sendErr := ch.send(event{Kind: evKeyringLocked, Text: text}); sendErr != nil {
		log.Printf("%s; and the daemon could not be told: %v", text, sendErr)
	}
}

// handOverAs is unlockKeyringAs on its locked thread.
func handOverAs(a account, env []string, password, previous []byte) error {
	if err := becomeOnThread(a); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), keyringWait)
	defer cancel()
	return auth.AwaitLoginKeyring(ctx, lookupIn(env), password, previous)
}

// becomeOnThread gives the calling OS thread a's groups, gid and uid,
// real, effective and saved: the thread can never take root's back, so
// its goroutine must stay locked to it, which ends the thread with the
// goroutine. The raw system calls change this thread alone, where the
// syscall package's Setuid and the others change every thread.
func becomeOnThread(a account) error {
	var groups unsafe.Pointer
	if len(a.groups) > 0 {
		groups = unsafe.Pointer(&a.groups[0]) //nolint:gosec // setgroups(2) reads len(a.groups) gid_t values here
	}
	if _, _, errno := unix.RawSyscall(unix.SYS_SETGROUPS, uintptr(len(a.groups)), uintptr(groups), 0); errno != 0 {
		return fmt.Errorf("give the keyring's thread %s's groups: %w", a.name, errno)
	}
	if _, _, errno := unix.RawSyscall(unix.SYS_SETRESGID, uintptr(a.gid), uintptr(a.gid), uintptr(a.gid)); errno != 0 {
		return fmt.Errorf("give the keyring's thread %s's gid %d: %w", a.name, a.gid, errno)
	}
	if _, _, errno := unix.RawSyscall(unix.SYS_SETRESUID, uintptr(a.uid), uintptr(a.uid), uintptr(a.uid)); errno != 0 {
		return fmt.Errorf("give the keyring's thread %s's uid %d: %w", a.name, a.uid, errno)
	}
	return nil
}

// lookupIn is env, KEY=value pairs, as an xdg.Env: the last value of a
// key, as getenv(3) finds the first.
func lookupIn(env []string) xdg.Env {
	return func(name string) string {
		for _, kv := range slices.Backward(env) {
			if k, v, ok := strings.Cut(kv, "="); ok && k == name {
				return v
			}
		}
		return ""
	}
}

const fingerprintDialTimeout = 5 * time.Second

func (o WorkerOptions) authenticate(user string, ask auth.AskUntil, ch *channel) (*auth.Login, error) {
	switch {
	case o.Autologin:
		return auth.Passwordless(o.Service, user)
	case o.FingerprintService != "":
		return auth.AuthenticateOrFingerprint(auth.FingerprintServices{Typed: o.Service, Fingerprint: o.FingerprintService}, user, ask, fingerprintScanner(ch))
	}
	return auth.Authenticate(o.Service, user, func(p auth.Prompt) (string, bool) { return ask(p, nil) })
}

func logPasswordless(login *auth.Login) {
	switch {
	case login.Typed():
	case login.HasSecret():
		log.Printf("dm: %s logged in without a password: the disk passphrase from the kernel keyring goes to the login keyring", login.User())
	default:
		log.Printf("dm: %s logged in without a password and with no disk passphrase in the kernel keyring, so the login keyring stays locked until the lock screen opens it", login.User())
	}
}

func fingerprintScanner(ch *channel) auth.Scanner {
	return func(user string) (<-chan struct{}, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		matched := make(chan struct{})
		match := sync.OnceFunc(func() { close(matched) })
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			err := verifyFingerprint(ctx, user, func(r fprint.Result) {
				if ctx.Err() != nil {
					return
				}
				if r == fprint.Match {
					log.Printf("dm: the fingerprint of %s matched", user)
					match()
					return
				}
				if err := ch.send(event{Kind: evFingerprint, Fingerprint: r}); err != nil {
					log.Printf("dm: tell the daemon what the fingerprint reader did: %v", err)
				}
			})
			if err != nil {
				log.Printf("dm: the fingerprint of %s: %v; the password alone logs in", user, err)
			}
		}()
		return matched, func() {
			cancel()
			<-ended
		}
	}
}

func verifyFingerprint(ctx context.Context, user string, onResult func(fprint.Result)) error {
	conn, err := dbusx.DialSystem(ctx, fingerprintDialTimeout)
	if err != nil {
		return fmt.Errorf("reach fprintd on the system bus: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("dm: close the system bus connection fprintd was reached on: %v", err)
		}
	}()
	return fprint.VerifyWhileLidOpen(ctx, conn, user, onResult)
}

func askThrough(ch *channel, requests <-chan incoming) auth.AskUntil {
	return func(p auth.Prompt, withdrawn <-chan struct{}) (string, bool) {
		select {
		case <-withdrawn:
			return "", false
		default:
		}
		if err := ch.send(event{Kind: evPrompt, Prompt: &p}); err != nil {
			return "", false
		}
		select {
		case in, ok := <-requests:
			if !ok || in.err != nil || in.req.Op != opAnswer || in.req.Answer == nil {
				return "", false
			}
			return *in.req.Answer, true
		case <-withdrawn:
			return "", false
		}
	}
}

func awaitStart(requests <-chan incoming) (request, error) {
	for in := range requests {
		if in.err != nil {
			return request{}, in.err
		}
		if in.req.Op != opAnswer {
			return in.req, nil
		}
	}
	return request{}, errClosed
}

// RunSession is one login's worker, serving the daemon over rw: it
// authenticates the user the daemon names, relaying PAM's prompts, and
// once told to start, opens the session, runs the command as the user
// until it exits, and closes the session. It must run on the process's
// main thread, as auth.Login says.
func RunSession(o WorkerOptions, rw io.ReadWriter) error {
	ch := newChannel(rw)
	done := make(chan struct{})
	defer close(done)
	requests := ch.requests(done)
	first, ok := <-requests
	if !ok {
		return errClosed
	}
	if first.err != nil {
		return first.err
	}
	req := first.req
	if req.Op != opAuth || req.User == "" {
		return fmt.Errorf("dm: a session worker's first request is %q, want auth", req.Op)
	}
	login, err := o.authenticate(req.User, askThrough(ch, requests), ch)
	if err != nil {
		log.Printf("dm: login of %s: %v", req.User, err)
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	defer func() {
		if err := login.Close(); err != nil {
			log.Printf("dm: session of %s: %v", req.User, err)
		}
	}()
	logPasswordless(login)
	secret, previous := login.TakeSecret(), login.TakePrevious()
	defer func() { clear(secret); clear(previous) }()
	if err := ch.send(event{Kind: evAuthenticated}); err != nil {
		return err
	}
	if req, err = awaitStart(requests); err != nil {
		return err
	}
	if req.Op != opStart || len(req.Cmd) == 0 {
		return nil
	}
	a, err := lookupAccount(login.User())
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	sessionEnv, err := o.loginEnv(req.Env)
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	pamEnv, err := login.OpenSession(sessionEnv)
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	sessionID, err := openedSession(pamEnv)
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	env := a.environ(pamEnv, req.Env)
	argv := append([]string{"/bin/sh", "-c", profileScript, "sh"}, req.Cmd...)
	held, err := holdSessionLock(o.RuntimeDir, o.VT)
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	defer func() { _ = held.Close() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stop)
	cmd, err := o.startAs(sessionID, a, argv, env)
	if err != nil {
		return ch.send(event{Kind: evFailed, Text: err.Error()})
	}
	if err := ch.send(event{Kind: evStarted, UID: a.uid, SessionID: string(sessionID)}); err != nil {
		log.Printf("dm: tell the daemon the session started: %v", err)
	}
	if secret != nil {
		unlocked := unlockKeyringAs(a, env, secret, previous)
		secret, previous = nil, nil
		go reportKeyring(ch, a.name, unlocked)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-exited:
	case <-stop:
		waitErr = stopGroup("the session of "+a.name, cmd, exited)
	}
	text := "exited"
	if waitErr != nil {
		text = waitErr.Error()
	}
	log.Printf("dm: the session of %s ended: %s", a.name, text)
	return ch.send(event{Kind: evEnded, Text: text})
}
