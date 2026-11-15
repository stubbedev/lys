package auth

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/login/internal/fprint"
)

// GreetdSockEnv names the IPC socket greetd hands its greeter.
const GreetdSockEnv = "GREETD_SOCK"

// Greetd is the pre-login backend (greetd_backend.rs): an unprivileged
// greeter does not talk to PAM, greetd does, over its IPC socket — a
// native-endian u32 length prefix before each JSON body. One Run is the
// whole login: create the session, answer every auth message, and on
// success start the session; greetd then replaces the greeter.
type Greetd struct {
	conn   io.ReadWriter
	start  Start
	frames <-chan greetdFrame
}

type greetdFrame struct {
	resp greetdResponse
	err  error
}

// SessionTypeEnv is the variable pam_systemd registers a session's type
// with, and sessions read to tell what they run on.
const SessionTypeEnv = "XDG_SESSION_TYPE"

// Session is the command and the environment a login starts.
type Session struct {
	Cmd, Env []string
}

// Start is what a successful login does: Session, read at start_session,
// says what it starts, and Before runs between the authentication and
// start_session.
type Start struct {
	Session func() Session
	Before  func()
}

// NewGreetdFromEnv connects to the socket in $GREETD_SOCK.
func NewGreetdFromEnv(start Start) (*Greetd, error) {
	path := os.Getenv(GreetdSockEnv)
	if path == "" {
		return nil, fmt.Errorf("%s is not set; not running under greetd", GreetdSockEnv)
	}
	conn, err := net.Dial("unix", path) //nolint:gosec // greetd names its own socket for the greeter it spawned
	if err != nil {
		return nil, err
	}
	return NewGreetd(conn, start), nil
}

// NewGreetd runs the protocol over an established connection.
func NewGreetd(conn io.ReadWriter, start Start) *Greetd {
	return &Greetd{conn: conn, start: start}
}

// Close closes the connection when it is closable.
func (g *Greetd) Close() error {
	if c, ok := g.conn.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// The requests, shaped like serde's internally tagged enum: the
// "type" tag first, then exactly the variant's fields.
type (
	createSessionRequest struct {
		Type     string `json:"type"`
		Username string `json:"username"`
	}
	// postResponseRequest's Response is null for info and error
	// messages.
	postResponseRequest struct {
		Type     string  `json:"type"`
		Response *string `json:"response"`
	}
	startSessionRequest struct {
		Type string   `json:"type"`
		Cmd  []string `json:"cmd"`
		Env  []string `json:"env"`
	}
	cancelSessionRequest struct {
		Type string `json:"type"`
	}
)

func createSession(username string) createSessionRequest {
	return createSessionRequest{Type: "create_session", Username: username}
}

func postResponse(response *string) postResponseRequest {
	return postResponseRequest{Type: "post_auth_message_response", Response: response}
}

// startSession never sends null arrays: serde's Vec is always a list.
func startSession(cmd, env []string) startSessionRequest {
	if cmd == nil {
		cmd = []string{}
	}
	if env == nil {
		env = []string{}
	}
	return startSessionRequest{Type: "start_session", Cmd: cmd, Env: env}
}

var cancelSession = cancelSessionRequest{Type: "cancel_session"}

// greetdResponse is one decoded response.
type greetdResponse struct {
	Type            string        `json:"type"`
	ErrorType       string        `json:"error_type"`
	Description     string        `json:"description"`
	AuthMessageType string        `json:"auth_message_type"`
	AuthMessage     string        `json:"auth_message"`
	Fingerprint     fprint.Result `json:"fingerprint"`
}

const greetdFingerprint = "fingerprint"

// validate rejects what serde's enums would: unknown variants and
// unknown error or message kinds.
func (r greetdResponse) validate() error {
	switch r.Type {
	case "success":
		return nil
	case "error":
		if r.ErrorType != "auth_error" && r.ErrorType != "error" {
			return fmt.Errorf("greetd: unknown error_type %q", r.ErrorType)
		}
		return nil
	case "auth_message":
		if _, err := greetdPrompt(r.AuthMessageType, r.AuthMessage); err != nil {
			return err
		}
		return nil
	case greetdFingerprint:
		return checkFingerprintStatus(r.Fingerprint)
	}
	return fmt.Errorf("greetd: unknown response type %q", r.Type)
}

func checkFingerprintStatus(r fprint.Result) error {
	if _, err := r.MarshalText(); err != nil {
		return fmt.Errorf("greetd: a fingerprint status: %w", err)
	}
	if r == fprint.Match {
		return errors.New("greetd: a fingerprint match is a success, not a status")
	}
	return nil
}

// greetdPrompt maps an auth message to the UI prompt.
func greetdPrompt(kind, text string) (Prompt, error) {
	switch kind {
	case "visible":
		return Prompt{Kind: PromptVisible, Text: text}, nil
	case "secret":
		return Prompt{Kind: PromptSecret, Text: text}, nil
	case "info":
		return Prompt{Kind: PromptInfo, Text: text}, nil
	case "error":
		return Prompt{Kind: PromptError, Text: text}, nil
	}
	return Prompt{}, fmt.Errorf("greetd: unknown auth_message_type %q", kind)
}

// maxGreetdFrame bounds a frame's length: greetd messages are small,
// and a peer must not make the other side allocate gigabytes.
const maxGreetdFrame = 1 << 20

// writeGreetdFrame writes one length-prefixed JSON message.
func writeGreetdFrame(w io.Writer, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	size, err := intconv.Exact[uint32](len(payload))
	if err != nil {
		return fmt.Errorf("message too large: %w", err)
	}
	frame := binary.NativeEndian.AppendUint32(make([]byte, 0, 4+len(payload)), size)
	frame = append(frame, payload...)
	_, err = w.Write(frame)
	return err
}

// readGreetdFrame reads one length-prefixed JSON message into v.
func readGreetdFrame(r io.Reader, v any) error {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return err
	}
	n := binary.NativeEndian.Uint32(head[:])
	if n > maxGreetdFrame {
		return fmt.Errorf("greetd: a %d-byte message is over the %d-byte limit", n, maxGreetdFrame)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}

// cancel cancels the session and reads greetd's answer, so the
// connection is idle again rather than closed under a reply in flight.
func (g *Greetd) cancel(ask Ask) error {
	if err := g.send(cancelSession); err != nil {
		return fmt.Errorf("greetd: cancel the session: %w", err)
	}
	resp, err := g.next(ask)
	if err != nil {
		return fmt.Errorf("greetd: the answer to cancel_session: %w", err)
	}
	if resp.Type != "success" {
		return fmt.Errorf("greetd: cancel_session refused: %s", resp.Description)
	}
	return nil
}

// send writes one request.
func (g *Greetd) send(req any) error { return writeGreetdFrame(g.conn, req) }

func (g *Greetd) readFrames(done <-chan struct{}) <-chan greetdFrame {
	frames := make(chan greetdFrame)
	go func() {
		defer close(frames)
		for {
			var resp greetdResponse
			err := readGreetdFrame(g.conn, &resp)
			if err == nil {
				err = resp.validate()
			}
			select {
			case frames <- greetdFrame{resp: resp, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return frames
}

func (g *Greetd) receive() (greetdResponse, error) {
	f, ok := <-g.frames
	if !ok {
		return greetdResponse{}, errors.New("greetd: the connection ended")
	}
	return f.resp, f.err
}

func (g *Greetd) next(ask Ask) (greetdResponse, error) {
	for {
		resp, err := g.receive()
		if err != nil || resp.Type != greetdFingerprint {
			return resp, err
		}
		ask(fingerprintPrompt(resp))
	}
}

func fingerprintPrompt(resp greetdResponse) Prompt {
	return Prompt{Kind: PromptFingerprint, Fingerprint: resp.Fingerprint}
}

// Run implements Conversation: create_session, one
// post_auth_message_response per auth message, then start_session.
// A greetd error or a cancelled input prompt cancels the session.
func (g *Greetd) Run(username string, ask Ask) error {
	// greetd needs the username up front; ask for it when not given.
	if username == "" {
		u, ok := ask(Prompt{Kind: PromptVisible, Text: "Username"})
		if !ok {
			return ErrCancelled
		}
		username = u
	}
	done := make(chan struct{})
	defer close(done)
	g.frames = g.readFrames(done)
	if err := g.send(createSession(username)); err != nil {
		return err
	}
	resp, err := g.next(ask)
	for err == nil && resp.Type == "auth_message" {
		resp, err = g.answer(resp, ask)
	}
	if err != nil {
		return err
	}
	if resp.Type == "error" {
		return errors.Join(greetdFailure(resp.Description), g.cancel(ask))
	}

	// Authenticated: hand off to the session; greetd replaces us.
	if g.start.Before != nil {
		g.start.Before()
	}
	session := g.start.Session()
	if err := g.send(startSession(session.Cmd, session.Env)); err != nil {
		return err
	}
	resp, err = g.next(ask)
	if err != nil {
		return err
	}
	switch resp.Type {
	case "success":
		return nil
	case "error":
		return greetdFailure(resp.Description)
	}
	log.Printf("greetd: unexpected response to start_session: %+v", resp)
	return errors.New("unexpected response to start_session")
}

func (g *Greetd) answer(resp greetdResponse, ask Ask) (greetdResponse, error) {
	prompt, _ := greetdPrompt(resp.AuthMessageType, resp.AuthMessage)
	if !prompt.WantsInput() {
		answer, ok := ask(prompt)
		return g.respond(answer, ok, ask)
	}
	answered := make(chan reply, 1)
	go func() {
		answer, ok := ask(prompt)
		answered <- reply{text: answer, ok: ok}
	}()
	for {
		select {
		case r := <-answered:
			if !r.ok {
				return greetdResponse{}, errors.Join(ErrCancelled, g.cancel(ask))
			}
			return g.respond(r.text, true, ask)
		case f, open := <-g.frames:
			if !open {
				return greetdResponse{}, errors.New("greetd: the connection ended")
			}
			switch {
			case f.err != nil:
				return greetdResponse{}, f.err
			case f.resp.Type == greetdFingerprint:
				ask(fingerprintPrompt(f.resp))
			case f.resp.Type == "auth_message":
				return greetdResponse{}, fmt.Errorf("greetd: the prompt %q came while %q waits for its answer", f.resp.AuthMessage, prompt.Text)
			default:
				return f.resp, nil
			}
		}
	}
}

func (g *Greetd) respond(answer string, ok bool, ask Ask) (greetdResponse, error) {
	var response *string
	if ok {
		response = &answer
	}
	if err := g.send(postResponse(response)); err != nil {
		return greetdResponse{}, err
	}
	return g.next(ask)
}

// greetdFailure is a greetd error's description as an error, a
// RefusedError when PAM refused the login.
func greetdFailure(description string) error {
	if refused, ok := parseRefused(description); ok {
		return refused
	}
	return errors.New(description)
}
