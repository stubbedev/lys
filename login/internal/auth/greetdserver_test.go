package auth

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"net"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The server half speaks to the client half: a login through a prompt,
// a failed one, and a start_session carrying the command.
func TestGreetdServerAndClientAgree(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	done := make(chan error, 1)
	var started GreetdRequest
	go func() {
		done <- func() error {
			req, err := ReadGreetdRequest(server)
			if err != nil || req.Type != GreetdCreateSession || req.Username != "alice" {
				return errors.Join(err, errors.New("want create_session for alice"))
			}
			if err := WriteGreetdPrompt(server, Prompt{Kind: PromptSecret, Text: "Password:"}); err != nil {
				return err
			}
			req, err = ReadGreetdRequest(server)
			if err != nil || req.Type != GreetdPostResponse || req.Response == nil || *req.Response != "pw" {
				return errors.Join(err, errors.New("want the password"))
			}
			if err := WriteGreetdSuccess(server); err != nil {
				return err
			}
			if started, err = ReadGreetdRequest(server); err != nil {
				return err
			}
			return WriteGreetdSuccess(server)
		}()
	}()
	g := NewGreetd(client, Start{Session: func() Session { return Session{Cmd: []string{"lys-wm", "--systemd"}, Env: []string{"A=1"}} }})
	var seen []Prompt
	if err := g.Run("alice", answering("pw", &seen)); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if started.Type != GreetdStartSession || !slices.Equal(started.Cmd, []string{"lys-wm", "--systemd"}) || !slices.Equal(started.Env, []string{"A=1"}) {
		t.Errorf("start_session = %+v", started)
	}
	if len(seen) != 1 || seen[0].Kind != PromptSecret {
		t.Errorf("prompts = %+v", seen)
	}
}

func TestGreetdServerErrorsReachTheClient(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close(); _ = client.Close() })
	cancelled := make(chan GreetdRequest, 1)
	go func() {
		_, _ = ReadGreetdRequest(server)
		_ = WriteGreetdError(server, true, "Authentication failure")
		req, _ := ReadGreetdRequest(server)
		cancelled <- req
		_ = WriteGreetdSuccess(server)
	}()
	err := NewGreetd(client, Start{Session: func() Session { return Session{} }}).Run("alice", answering("pw", &[]Prompt{}))
	if err == nil || err.Error() != "Authentication failure" {
		t.Errorf("Run = %v, want the server's description alone", err)
	}
	if req := <-cancelled; req.Type != GreetdCancelSession {
		t.Errorf("after the error the client sent %+v, want cancel_session", req)
	}
}

func TestReadGreetdRequestRefusesBadFrames(t *testing.T) {
	frame := func(body string) *bytes.Buffer {
		b := binary.NativeEndian.AppendUint32(nil, uint32(len(body)))
		return bytes.NewBuffer(append(b, body...))
	}
	if _, err := ReadGreetdRequest(frame(`{"type":"reboot"}`)); err == nil {
		t.Error("an unknown request type was accepted")
	}
	if _, err := ReadGreetdRequest(bytes.NewBuffer(binary.NativeEndian.AppendUint32(nil, maxGreetdFrame+1))); !errors.Is(err, errGreetdFrameTooLarge) {
		t.Errorf("an oversized frame = %v, want %v", err, errGreetdFrameTooLarge)
	}
	if err := WriteGreetdPrompt(&bytes.Buffer{}, Prompt{Kind: 99}); err == nil {
		t.Error("a prompt of no known kind was sent")
	}
}

func TestAGreetdFrameAtTheLimitIsWrittenAndReadBack(t *testing.T) {
	var buf bytes.Buffer
	overhead := len(`""`)
	body := strings.Repeat("a", maxGreetdFrame-overhead)
	if err := writeGreetdFrame(&buf, body); err != nil {
		t.Fatalf("a %d-byte message: %v", maxGreetdFrame, err)
	}
	var got string
	if err := readGreetdFrame(&buf, &got); err != nil || got != body {
		t.Fatalf("read back %d bytes, %v; want the message at the limit", len(got), err)
	}
}

func TestAnOversizedGreetdFrameIsNeverWritten(t *testing.T) {
	var buf bytes.Buffer
	err := writeGreetdFrame(&buf, strings.Repeat("a", maxGreetdFrame))
	if !errors.Is(err, errGreetdFrameTooLarge) {
		t.Fatalf("a message over the limit = %v, want %v", err, errGreetdFrameTooLarge)
	}
	if buf.Len() != 0 {
		t.Errorf("wrote %d bytes of a refused message", buf.Len())
	}
}

func TestAnOversizedGreetdLengthIsRefusedWithoutAllocatingIt(t *testing.T) {
	head := binary.NativeEndian.AppendUint32(nil, math.MaxUint32)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err := readGreetdFrame(bytes.NewReader(head), new(json.RawMessage))
	runtime.ReadMemStats(&after)
	if !errors.Is(err, errGreetdFrameTooLarge) {
		t.Fatalf("a %d-byte length = %v, want %v", uint32(math.MaxUint32), err, errGreetdFrameTooLarge)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew >= maxGreetdFrame {
		t.Errorf("refusing the length allocated %d bytes", grew)
	}
}
