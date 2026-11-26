package dochelper

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// ReadingMemory is what a helper serving one document a reader shows
// may hold.
var ReadingMemory = Memory{Hard: 1 << 30, Soft: 384 << 20}

// RequestTimeout is how long a helper may take over one request before
// it is killed and the request fails with ErrTimeout; the thumbnailer
// gives it as long for one thumbnail.
const RequestTimeout = 10 * time.Second

// OutputBytes is how much of what a helper prints is kept for its
// error.
const OutputBytes = 4 << 10

// Head keeps the first Limit bytes written to it and drops the rest.
type Head struct {
	Limit int
	b     []byte
}

func (h *Head) Write(p []byte) (int, error) {
	h.b = append(h.b, p[:min(len(p), max(h.Limit-len(h.b), 0))]...)
	return len(p), nil
}

// Bytes is what was kept.
func (h *Head) Bytes() []byte { return h.b }

// Process is a started serving helper: requests go to its standard
// input, replies come from its standard output, and the start of what
// it prints on its standard error says why it died.
type Process struct {
	cmd    *exec.Cmd
	in     *os.File
	out    *os.File
	r      *bufio.Reader
	w      *bufio.Writer
	stderr Head

	mu     sync.Mutex
	reaped bool
	once   sync.Once
	exit   error
}

// StartProcess starts cmd, made by Command, with pipes to its standard
// input and output.
func StartProcess(cmd *exec.Cmd) (*Process, error) {
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("%s: a pipe to its input: %w", Program, err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%s: a pipe from its output: %w", Program, err), inR.Close(), inW.Close())
	}
	p := &Process{cmd: cmd, in: inW, out: outR, r: bufio.NewReader(outR), w: bufio.NewWriter(inW), stderr: Head{Limit: OutputBytes}}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, &p.stderr
	err = cmd.Start()
	err = errors.Join(err, inR.Close(), outW.Close())
	if err != nil {
		return nil, errors.Join(fmt.Errorf("%s: starting %s: %w", Program, cmd.Path, err), inW.Close(), outR.Close())
	}
	return p, nil
}

// Starter is what starts the helper at path (Locate's) serving as s
// says, the way the thumbnailer starts it.
func Starter(path string, s Serving) func() (*Process, error) {
	return func() (*Process, error) {
		return StartProcess(Command(context.Background(), path, s.Args()))
	}
}

// Kill kills the helper's process group, unless it was reaped.
func (p *Process) Kill() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reaped {
		return
	}
	if err := killGroup(p.cmd); err != nil {
		log.Print(err)
	}
}

// Wait closes the helper's input, waits for it to exit and is the
// error its exit tells: one of the errors for its status, ErrTooLarge
// for the runtime dying out of memory, else what went wrong and what
// it printed; nil for a helper that exited 0.
func (p *Process) Wait() error {
	p.once.Do(func() {
		cerr := p.in.Close()
		err := p.cmd.Wait()
		p.mu.Lock()
		p.reaped = true
		p.mu.Unlock()
		cerr = errors.Join(cerr, p.out.Close())
		output := bytes.TrimSpace(p.stderr.Bytes())
		code := p.cmd.ProcessState.ExitCode()
		switch told := ExitError(code, output); {
		case told != nil:
			p.exit = fmt.Errorf("%w: %s exited with status %d: %s", told, Program, code, output)
		case err != nil:
			p.exit = fmt.Errorf("%s: %w: %s", Program, err, output)
		default:
			p.exit = cerr
		}
	})
	return p.exit
}

// Client reads one document through a serving helper it starts on the
// first request and again on the first after the helper died. Requests
// take turns, each within the timeout, past which the helper is killed
// whole and the request fails with ErrTimeout. A helper that dies over
// a request fails it with the error its exit tells (ErrTooLarge for one
// that ran out of memory); one that answers outside the protocol is
// killed. Neither is retried: the next request starts a new helper.
type Client struct {
	start   func() (*Process, error)
	path    string
	timeout time.Duration

	mu     sync.Mutex
	proc   *Process
	info   Info
	closed bool
}

// NewClient is a client reading the document at path through the
// helpers start starts, each request within timeout.
func NewClient(start func() (*Process, error), path string, timeout time.Duration) *Client {
	return &Client{start: start, path: path, timeout: timeout}
}

// errClosed is a request to a closed client.
var errClosed = errors.New(Program + ": the document is closed")

// Info is what the document says of itself.
func (c *Client) Info() (Info, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensure(); err != nil {
		return Info{}, err
	}
	return c.info, nil
}

// Render draws tile t. A tile drawn only in part is returned with the
// error saying what was left out.
func (c *Client) Render(t Tile) (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensure(); err != nil {
		return nil, err
	}
	if err := c.info.CheckTile(t); err != nil {
		return nil, err
	}
	body, err := c.request(kindRender, t)
	if err != nil {
		return nil, err
	}
	img, note, err := readTile(body, t.Rect.Size())
	if err != nil {
		return nil, errors.Join(err, c.discard())
	}
	if note != "" {
		return img, fmt.Errorf("%s: page %d drawn in part: %s", Program, t.Page+1, note)
	}
	return img, nil
}

// Text is what page (from zero) says.
func (c *Client) Text(page int) (Text, error) {
	var text Text
	err := c.pageRequest(kindText, page, &text, text.validate)
	return text, err
}

// Links are where page (from zero) leads.
func (c *Client) Links(page int) ([]Link, error) {
	var links []Link
	err := c.pageRequest(kindLinks, page, &links, func() error { return validateLinks(links, c.info.Pages()) })
	return links, err
}

// pageRequest asks about page with a request of kind k, decoding the
// reply into v and checking it with validate.
func (c *Client) pageRequest(k kind, page int, v any, validate func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensure(); err != nil {
		return err
	}
	if err := c.info.checkPage(page); err != nil {
		return err
	}
	body, err := c.request(k, pageRequest{Page: page})
	if err != nil {
		return err
	}
	if err := readJSON(k, body, v); err != nil {
		return errors.Join(err, c.discard())
	}
	if err := validate(); err != nil {
		return errors.Join(err, c.discard())
	}
	return nil
}

// Close has the helper close the document and exit, and refuses every
// request after.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.proc == nil {
		return nil
	}
	p := c.proc
	_, err := c.exchange(kindClose, nil)
	if c.proc == nil {
		return err
	}
	c.proc = nil
	return errors.Join(err, p.Wait())
}

// ensure starts a helper and has it open the document, unless one is
// running.
func (c *Client) ensure() error {
	if c.closed {
		return errClosed
	}
	if c.proc != nil {
		return nil
	}
	p, err := c.start()
	if err != nil {
		return err
	}
	c.proc = p
	body, err := c.request(kindOpen, openRequest{Path: c.path})
	if err == nil {
		var info Info
		if err = readJSON(kindOpen, body, &info); err == nil {
			err = info.validate()
		}
		c.info = info
	}
	if err != nil {
		return fmt.Errorf("opening %s: %w", c.path, errors.Join(err, c.discard()))
	}
	return nil
}

// request sends a request of kind k whose body is v as JSON and is the
// body of its reply, or the error the helper failed it with.
func (c *Client) request(k kind, v any) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%s: encoding a %v request: %w", Program, k, err)
	}
	return c.exchange(k, body)
}

// exchange sends one request frame and reads its reply within the
// timeout. A helper that does not answer in time is killed and the
// request is ErrTimeout; one that answers outside the protocol is
// killed and the request is errProtocol; one that dies is reaped and
// the request fails with the error its exit tells.
func (c *Client) exchange(k kind, body []byte) ([]byte, error) {
	p := c.proc
	var late atomic.Bool
	timer := time.AfterFunc(c.timeout, func() {
		late.Store(true)
		p.Kill()
	})
	var parts [][]byte
	if body != nil {
		parts = [][]byte{body}
	}
	err := writeFrame(p.w, k, parts...)
	var rk kind
	var reply []byte
	if err == nil {
		rk, reply, err = readFrame(p.r)
	}
	if !timer.Stop() || late.Load() {
		return nil, errors.Join(fmt.Errorf("%w: a %v request over %v", ErrTimeout, k, c.timeout), c.discard())
	}
	if err == nil && rk != k && rk != kindFailed {
		err = fmt.Errorf("%w: a %v reply to a %v request", errProtocol, rk, k)
	}
	if errors.Is(err, errProtocol) {
		return nil, errors.Join(fmt.Errorf("a %v request: %w", k, err), c.discard())
	}
	if err != nil {
		if exit := c.reap(); exit != nil {
			return nil, fmt.Errorf("a %v request: %w", k, exit)
		}
		return nil, fmt.Errorf("a %v request: %w", k, err)
	}
	if rk == kindFailed {
		return nil, readFailure(reply)
	}
	return reply, nil
}

// reap waits for the helper that stopped answering, which the next
// request starts again, and is the error its exit tells.
func (c *Client) reap() error {
	p := c.proc
	c.proc = nil
	p.Kill()
	return p.Wait()
}

// discard kills the helper, if one runs, which the next request starts
// again: one that ran out of time or answered outside the protocol, or
// that holds a document that did not open. It is how the helper
// exited, for the failure to carry.
func (c *Client) discard() error {
	p := c.proc
	if p == nil {
		return nil
	}
	c.proc = nil
	p.Kill()
	return p.Wait()
}
