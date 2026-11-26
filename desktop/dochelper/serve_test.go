package dochelper

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/testbuild"
	"github.com/stubbedev/lys/lib/testhome"
)

// The pages of the fake document, by what asking for them does: Plain
// answers, Hangs never does, Crashes exits, Grows allocates past the
// helper's memory, Garbles writes bytes outside the protocol and Left
// is drawn in part.
const (
	pagePlain = iota
	pageHangs
	pageCrashes
	pageGrows
	pageGarbles
	pageLeft
	fakePages
)

// fakeMemory is what the fake helper holds itself to: room for the
// race detector's own allocator in a race build.
var fakeMemory = map[bool]Memory{false: {Hard: 256 << 20, Soft: 64 << 20}, true: {Hard: 4 << 30, Soft: 1 << 30}}[testbuild.Race]

// startsFile, beside the fake helper, gets a line each time it starts.
const startsFile = "starts"

// TestMain runs the package against a scratch home; started as
// lys-docthumb, the test binary is a helper serving the fake document.
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == Program {
		os.Exit(fakeHelper(os.Args[1:]))
	}
	testhome.Run(m)
}

func fakeHelper(args []string) int {
	s, ok, err := ParseServing(args)
	if !ok || err != nil {
		fmt.Fprintln(os.Stderr, "fake helper:", ok, err)
		return 1
	}
	if err := s.Memory.Limit(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	starts, err := os.OpenFile(filepath.Join(filepath.Dir(os.Args[0]), startsFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err == nil {
		_, err = fmt.Fprintln(starts, os.Getpid())
		err = errors.Join(err, starts.Close())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := Serve(os.Stdin, os.Stdout, &fake{memory: s.Memory.Hard}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ExitCode(err)
	}
	return 0
}

// fake is a document of fakePages square pages, each drawn in a grey
// of its number, and saying "page N".
type fake struct {
	memory int64
	opened string
}

var fakeInfo = Info{
	Sizes:   []Size{{100, 100}, {100, 100}, {100, 100}, {100, 100}, {100, 100}, {100, 100}},
	Labels:  []string{"i", "ii", "", "", "", ""},
	Outline: []Outline{{Title: "Plain", Page: pagePlain, Children: []Outline{{Title: "Left", Page: pageLeft}}}, {Title: "Out", Page: -1, URI: "https://example.org/"}},
}

func (f *fake) Open(path string) (Info, error) {
	switch filepath.Base(path) {
	case "damaged.pdf":
		return Info{}, fmt.Errorf("%w: a test document", ErrDamaged)
	case "empty.pdf":
		return Info{}, nil
	}
	f.opened = path
	return fakeInfo, nil
}

func (f *fake) Render(t Tile) (*image.RGBA, error) {
	switch t.Page {
	case pageHangs:
		time.Sleep(time.Hour)
	case pageCrashes:
		os.Exit(1)
	case pageGrows:
		buf := make([]byte, 2*f.memory)
		return nil, fmt.Errorf("%d", buf[len(buf)-1])
	case pageGarbles:
		_, _ = os.Stdout.WriteString("\x00\x00\x00\x01\xffjunk")
	}
	img := image.NewRGBA(image.Rectangle{Max: t.Rect.Size()})
	grey := color.RGBA{uint8(40 * t.Page), uint8(40 * t.Page), uint8(40 * t.Page), 255}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = grey.R, grey.G, grey.B, grey.A
	}
	if t.Page == pageLeft {
		return img, errors.New("a shading was left out")
	}
	return img, nil
}

func (f *fake) Text(page int) (Text, error) {
	var line Line
	for i, r := range "page " + strconv.Itoa(page+1) {
		x := float64(10 * i)
		line.Chars = append(line.Chars, Char{Rune: r, Box: Box{X0: x, Y0: 10, X1: x + 8, Y1: 20}})
	}
	return Text{Lines: []Line{line}}, nil
}

func (f *fake) Links(page int) ([]Link, error) {
	if page != pagePlain {
		return nil, nil
	}
	return []Link{{Box: Box{0, 0, 50, 10}, Page: pageLeft}, {Box: Box{0, 50, 50, 60}, Page: -1, URI: "https://example.org/"}}, nil
}

func (f *fake) Close() error { return nil }

// frames is the requests, each kind and body, as frames.
func frames(t *testing.T, reqs ...any) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	for i := 0; i < len(reqs); i += 2 {
		k := reqs[i].(kind)
		var err error
		if reqs[i+1] == nil {
			err = writeFrame(w, k)
		} else {
			err = writeJSON(w, k, reqs[i+1])
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return &b
}

// replies reads every frame Serve wrote.
func replies(t *testing.T, out *bytes.Buffer) (kinds []kind, bodies [][]byte) {
	t.Helper()
	r := bufio.NewReader(out)
	for {
		k, body, err := readFrame(r)
		if errors.Is(err, io.EOF) {
			return kinds, bodies
		}
		if err != nil {
			t.Fatalf("reading a reply: %v", err)
		}
		kinds = append(kinds, k)
		bodies = append(bodies, body)
	}
}

// Every request reads back as the handler answered it: the document
// opened, a tile, a page's text and links, a failure with its error,
// and the close ending the serving.
func TestServingRoundTripsEveryMessage(t *testing.T) {
	tile := Tile{Page: pageLeft, Scale: 2, Rect: image.Rect(10, 20, 74, 52)}
	in := frames(t,
		kindOpen, openRequest{Path: "/doc.pdf"},
		kindRender, tile,
		kindText, pageRequest{Page: 2},
		kindLinks, pageRequest{Page: pagePlain},
		kindText, pageRequest{Page: fakePages},
		kindClose, nil,
		kindText, pageRequest{Page: 0},
	)
	var out bytes.Buffer
	h := &fake{}
	if err := Serve(in, &out, h); err != nil {
		t.Fatal(err)
	}
	if h.opened != "/doc.pdf" {
		t.Errorf("the handler opened %q", h.opened)
	}
	kinds, bodies := replies(t, &out)
	if want := []kind{kindOpen, kindRender, kindText, kindLinks, kindFailed, kindClose}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("the replies are %v, want %v: nothing after the close", kinds, want)
	}
	var info Info
	if err := readJSON(kindOpen, bodies[0], &info); err != nil || !reflect.DeepEqual(info, fakeInfo) {
		t.Errorf("the document holds %+v, %v, want %+v", info, err, fakeInfo)
	}
	img, note, err := readTile(bodies[1], tile.Rect.Size())
	if err != nil || note != "a shading was left out" || img.Rect.Size() != tile.Rect.Size() || img.Pix[0] != 40*pageLeft {
		t.Errorf("the tile is %v with note %q, %v", img.Rect, note, err)
	}
	var text Text
	if err := readJSON(kindText, bodies[2], &text); err != nil || len(text.Lines) != 1 || len(text.Lines[0].Chars) != 6 || text.Lines[0].Chars[5].Rune != '3' {
		t.Errorf("page 3 says %+v, %v", text, err)
	}
	var links []Link
	if err := readJSON(kindLinks, bodies[3], &links); err != nil || validateLinks(links, fakePages) != nil || len(links) != 2 {
		t.Errorf("page 1 leads %+v, %v", links, err)
	}
	if err := readFailure(bodies[4]); err == nil || !strings.Contains(err.Error(), "not one of") {
		t.Errorf("a page past the last = %v", err)
	}
}

// A document the handler fails to open, or opens with no pages, is a
// failure the request is answered with, and requests before an open
// and a second open fail without ending the serving.
func TestServingAnswersARequestOutOfTurnWithAFailure(t *testing.T) {
	in := frames(t,
		kindText, pageRequest{Page: 0},
		kindOpen, openRequest{Path: "/damaged.pdf"},
		kindOpen, openRequest{Path: "/empty.pdf"},
		kindOpen, openRequest{Path: "/doc.pdf"},
		kindOpen, openRequest{Path: "/doc.pdf"},
		kindRender, Tile{Page: 0, Scale: 1, Rect: image.Rect(0, 0, 101, 10)},
		kindRender, Tile{Page: 0, Scale: 0, Rect: image.Rect(0, 0, 1, 1)},
	)
	var out bytes.Buffer
	if err := Serve(in, &out, &fake{}); err != nil {
		t.Fatal(err)
	}
	kinds, bodies := replies(t, &out)
	if want := []kind{kindFailed, kindFailed, kindFailed, kindOpen, kindFailed, kindFailed, kindFailed}; !reflect.DeepEqual(kinds, want) {
		t.Fatalf("the replies are %v, want %v", kinds, want)
	}
	if err := readFailure(bodies[1]); !errors.Is(err, ErrDamaged) {
		t.Errorf("a damaged document = %v, want ErrDamaged", err)
	}
	if err := readFailure(bodies[2]); err == nil {
		t.Error("a document of no pages opened")
	}
}

// frame is one raw frame: length, kind and body as given.
func frame(n uint32, k kind, body string) []byte {
	b := binary.BigEndian.AppendUint32(nil, n)
	return append(append(b, byte(k)), body...)
}

// A frame cut short, empty, over the longest, of no kind or with a
// body the protocol does not allow ends the serving with errProtocol;
// a stream that ends between frames ends it cleanly.
func TestAMalformedFrameEndsTheServing(t *testing.T) {
	open := frame(uint32(1+len(`{"Path":"/d"}`)), kindOpen, `{"Path":"/d"}`)
	for name, in := range map[string][]byte{
		"a short header":   {0, 0, 0},
		"a short body":     frame(20, kindOpen, `{"Path":`),
		"an empty frame":   frame(0, kindOpen, ""),
		"an oversize one":  frame(maxFrame+1, kindOpen, ""),
		"no kind":          frame(1, 0, ""),
		"an unknown kind":  frame(1, kindFailed+1, ""),
		"a failure":        append(open, frame(2, kindFailed, "\x01")...),
		"not JSON":         frame(4, kindOpen, "{{{"),
		"an unknown field": frame(uint32(1+len(`{"Path":"/d","Mode":1}`)), kindOpen, `{"Path":"/d","Mode":1}`),
		"more after it":    frame(uint32(1+len(`{"Path":"/d"}{}`)), kindOpen, `{"Path":"/d"}{}`),
		"a close body":     append(open, frame(2, kindClose, "x")...),
	} {
		var out bytes.Buffer
		if err := Serve(bytes.NewReader(in), &out, &fake{}); !errors.Is(err, errProtocol) {
			t.Errorf("%s: serving = %v, want errProtocol", name, err)
		}
	}
	var out bytes.Buffer
	if err := Serve(bytes.NewReader(open), &out, &fake{}); err != nil {
		t.Errorf("a stream ending after a request: %v", err)
	}
}

// A reply the protocol does not allow is refused: a tile of another
// size or whose length does not add up, a failure of no status or of
// one no error has, and a description of a document that cannot be.
func TestAMalformedReplyIsRefused(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	good := bytes.Join(tileParts(img, "note"), nil)
	if _, note, err := readTile(good, image.Pt(4, 3)); err != nil || note != "note" {
		t.Fatalf("a good tile = %q, %v", note, err)
	}
	for name, body := range map[string][]byte{
		"another size": good,
		"cut short":    good[:len(good)-1],
		"no header":    good[:tileHeader-1],
	} {
		want := image.Pt(4, 3)
		if name == "another size" {
			want = image.Pt(3, 4)
		}
		if _, _, err := readTile(body, want); !errors.Is(err, errProtocol) {
			t.Errorf("a tile %s = %v, want errProtocol", name, err)
		}
	}
	for _, body := range [][]byte{nil, {99, 'x'}, {0, 'x'}} {
		if err := readFailure(body); !errors.Is(err, errProtocol) {
			t.Errorf("a failure %q = %v, want errProtocol", body, err)
		}
	}
	for code, want := range map[byte]error{3: ErrDamaged, 4: ErrEncrypted, 5: ErrTooLarge, 6: ErrUnsupported} {
		if err := readFailure([]byte{code, 'x'}); !errors.Is(err, want) {
			t.Errorf("a failure of status %d = %v, want %v", code, err, want)
		}
	}
	deep := []Outline{{Title: "x", Page: 0}}
	for range maxOutlineDepth {
		deep = []Outline{{Title: "x", Page: 0, Children: deep}}
	}
	sized := func(s ...Size) Info { return Info{Sizes: s} }
	for name, info := range map[string]Info{
		"no pages":      {},
		"a NaN page":    sized(Size{math.NaN(), 1}),
		"an empty page": sized(Size{0, 1}),
		"a huge page":   sized(Size{maxPagePoints + 1, 1}),
		"labels":        {Sizes: []Size{{1, 1}}, Labels: []string{"a", "b"}},
		"an outline":    {Sizes: []Size{{1, 1}}, Outline: []Outline{{Title: "x", Page: 1}}},
		"a deep one":    {Sizes: []Size{{1, 1}}, Outline: deep},
	} {
		if err := info.validate(); !errors.Is(err, errProtocol) {
			t.Errorf("%s: %v, want errProtocol", name, err)
		}
	}
	if err := (Text{Lines: []Line{{Chars: []Char{{Rune: 'a', Box: Box{0, 0, math.Inf(1), 1}}}}}}).validate(); !errors.Is(err, errProtocol) {
		t.Errorf("an infinite character box = %v", err)
	}
	for _, l := range []Link{{Page: -1}, {Page: 1, URI: "x"}, {Page: 5}, {Page: -2, URI: "x"}} {
		if err := validateLinks([]Link{l}, 2); !errors.Is(err, errProtocol) {
			t.Errorf("link %+v = %v, want errProtocol", l, err)
		}
	}
}

// A tile within its page at a scale within range is allowed; one out
// of the page, empty, of a page the document lacks, at no scale or
// larger than MaxSize is not, the last as ErrTooLarge.
func TestATileMustLieOnItsPage(t *testing.T) {
	info := Info{Sizes: []Size{{100, 50}}}
	if err := info.CheckTile(Tile{Scale: 2, Rect: image.Rect(0, 0, 200, 100)}); err != nil {
		t.Errorf("the whole page at 2: %v", err)
	}
	for name, tile := range map[string]Tile{
		"past the page": {Scale: 2, Rect: image.Rect(0, 0, 201, 100)},
		"empty":         {Scale: 2, Rect: image.Rect(5, 5, 5, 9)},
		"no page":       {Page: 1, Scale: 1, Rect: image.Rect(0, 0, 1, 1)},
		"no scale":      {Scale: 0, Rect: image.Rect(0, 0, 1, 1)},
		"NaN scale":     {Scale: math.NaN(), Rect: image.Rect(0, 0, 1, 1)},
		"over MaxScale": {Scale: MaxScale + 1, Rect: image.Rect(0, 0, 1, 1)},
	} {
		if err := info.CheckTile(tile); err == nil {
			t.Errorf("a tile %s was allowed", name)
		}
	}
	big := Info{Sizes: []Size{{1000, 1000}}}
	if err := big.CheckTile(Tile{Scale: 8, Rect: image.Rect(0, 0, MaxSize+1, 10)}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a tile over MaxSize = %v, want ErrTooLarge", err)
	}
}

// A serving command line reads back as written; one without the
// command is no serving, and one with memory out of order is refused.
func TestAServingCommandLineRoundTrips(t *testing.T) {
	want := Serving{Memory: ReadingMemory, FontDirs: []string{"/usr/share/fonts", "-odd"}}
	got, ok, err := ParseServing(want.Args())
	if !ok || err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseServing(%q) = %+v, %v, %v", want.Args(), got, ok, err)
	}
	if _, ok, err := ParseServing(Request{Input: "in", Output: "out", Size: 1, Memory: 1}.Args()); ok || err != nil {
		t.Errorf("a thumbnail request read as serving: %v, %v", ok, err)
	}
	for name, s := range map[string]Serving{
		"no memory":         {},
		"soft past hard":    {Memory: Memory{Hard: 1 << 20, Soft: 2 << 20}},
		"no soft":           {Memory: Memory{Hard: 1 << 20}},
		"a negative memory": {Memory: Memory{Hard: -1, Soft: -1}},
	} {
		if _, ok, err := ParseServing(s.Args()); !ok || err == nil {
			t.Errorf("%s: %q = %v, %v, want refused", name, s.Args(), ok, err)
		}
	}
	if _, _, err := ParseServing(append(want.Args(), "extra")); err == nil {
		t.Error("an argument after the flags was taken")
	}
}

// installedFake is the test binary installed as lys-docthumb, serving
// the fake document, in a fresh directory.
func installedFake(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), Program)
	if err := os.Symlink(exe, path); err != nil {
		t.Fatal(err)
	}
	return path
}

// starts are the PIDs the fake helper at path started as.
func starts(t *testing.T, path string) []int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), startsFile))
	if err != nil {
		t.Fatal(err)
	}
	var pids []int
	for f := range strings.FieldsSeq(string(data)) {
		pid, err := strconv.Atoi(f)
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	return pids
}

// alive reports whether a process pid is running.
func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }

func fakeClient(t *testing.T, timeout time.Duration) (*Client, string) {
	t.Helper()
	path, err := Locate(installedFake(t))
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(Starter(path, Serving{Memory: fakeMemory}), "/doc.pdf", timeout)
	t.Cleanup(func() { _ = c.Close() })
	return c, path
}

func plainTile(page int) Tile { return Tile{Page: page, Scale: 1, Rect: image.Rect(0, 0, 10, 10)} }

// A client starts its helper on the first request, reads the document
// through it, and once closed has it exit and refuses what follows.
func TestAClientReadsTheDocumentThroughItsHelper(t *testing.T) {
	c, path := fakeClient(t, RequestTimeout)
	info, err := c.Info()
	if err != nil || !reflect.DeepEqual(info, fakeInfo) {
		t.Fatalf("the document holds %+v, %v", info, err)
	}
	img, err := c.Render(Tile{Page: pagePlain, Scale: 1.5, Rect: image.Rect(3, 4, 13, 24)})
	if err != nil {
		t.Fatalf("a tile of page 1: %v", err)
	}
	if img.Rect != image.Rect(0, 0, 10, 20) || img.Pix[0] != 0 || img.Pix[3] != 255 {
		t.Errorf("a tile of page 1 = %v, %v", img.Rect, img.Pix[:4])
	}
	img, err = c.Render(plainTile(pageLeft))
	if img == nil || err == nil || !strings.Contains(err.Error(), "a shading was left out") {
		t.Errorf("a page drawn in part = %v, %v, want the tile and what it left out", img, err)
	}
	if text, err := c.Text(1); err != nil || len(text.Lines) != 1 {
		t.Errorf("page 2 says %+v, %v", text, err)
	}
	if links, err := c.Links(pagePlain); err != nil || len(links) != 2 || links[0].Page != pageLeft {
		t.Errorf("page 1 leads %+v, %v", links, err)
	}
	if _, err := c.Text(fakePages); err == nil {
		t.Error("a page past the last has text")
	}
	if _, err := c.Render(Tile{Page: 0, Scale: 1, Rect: image.Rect(0, 0, 101, 1)}); err == nil {
		t.Error("a tile past its page was drawn")
	}
	pids := starts(t, path)
	if len(pids) != 1 {
		t.Fatalf("the helper started %d times, want once", len(pids))
	}
	if err := c.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	if alive(pids[0]) {
		t.Error("the helper runs on after the close")
	}
	if _, err := c.Info(); !errors.Is(err, errClosed) {
		t.Errorf("a request after the close = %v, want errClosed", err)
	}
}

// A request the helper does not answer in time fails with ErrTimeout,
// the helper killed; the next request starts a new one.
func TestAHelperPastTheDeadlineIsKilledAndRestarted(t *testing.T) {
	const timeout = 300 * time.Millisecond
	c, path := fakeClient(t, timeout)
	if _, err := c.Info(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := c.Render(plainTile(pageHangs))
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("a hanging page = %v, want ErrTimeout", err)
	}
	if took := time.Since(start); took < timeout {
		t.Errorf("the request gave up after %v, before its %v", took, timeout)
	}
	pids := starts(t, path)
	if alive(pids[0]) {
		t.Error("the hanging helper was not killed")
	}
	if _, err := c.Render(plainTile(pagePlain)); err != nil {
		t.Fatalf("the request after: %v", err)
	}
	if n := len(starts(t, path)); n != 2 {
		t.Errorf("the helper started %d times, want again after the kill", n)
	}
}

// A helper that dies over a request, or answers outside the protocol,
// fails it with why, and is not asked again: the next request starts a
// new one, which answers. One out of memory fails it with ErrTooLarge,
// but in a race build, where the race detector's allocator dies first.
func TestAHelperThatDiesIsRestarted(t *testing.T) {
	c, path := fakeClient(t, RequestTimeout)
	for _, c2 := range []struct {
		page int
		want error
	}{
		{pageCrashes, nil},
		{pageGarbles, errProtocol},
		{pageGrows, map[bool]error{false: ErrTooLarge, true: nil}[testbuild.Race]},
	} {
		_, err := c.Render(plainTile(c2.page))
		if err == nil || errors.Is(err, ErrTimeout) || (c2.want != nil && !errors.Is(err, c2.want)) {
			t.Errorf("page %d = %v, want a failure that is %v", c2.page+1, err, c2.want)
		}
		if _, err := c.Render(plainTile(pagePlain)); err != nil {
			t.Fatalf("the request after page %d: %v", c2.page+1, err)
		}
	}
	pids := starts(t, path)
	if len(pids) != 4 {
		t.Errorf("the helper started %d times, want once and again after each of three deaths", len(pids))
	}
	for _, pid := range pids[:3] {
		if alive(pid) {
			t.Errorf("helper %d runs on after it failed", pid)
		}
	}
}

// A document the helper cannot open fails the request with its error,
// and every request after tries again.
func TestADocumentThatDoesNotOpenFailsEveryRequest(t *testing.T) {
	path, err := Locate(installedFake(t))
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(Starter(path, Serving{Memory: fakeMemory}), "/damaged.pdf", RequestTimeout)
	defer func() { _ = c.Close() }()
	for range 2 {
		if _, err := c.Info(); !errors.Is(err, ErrDamaged) {
			t.Errorf("opening a damaged document = %v, want ErrDamaged", err)
		}
	}
	if n := len(starts(t, path)); n != 2 {
		t.Errorf("the helper started %d times, want once a request", n)
	}
}

// Only lys-docthumb, at an absolute path, is ever started.
func TestOnlyLysDocthumbIsLocated(t *testing.T) {
	fake := installedFake(t)
	if got, err := Locate(fake); err != nil || got != fake {
		t.Errorf("Locate(%s) = %q, %v", fake, got, err)
	}
	for _, path := range []string{"/bin/sh", Program, filepath.Join(".", Program), filepath.Join(filepath.Dir(fake), "evince-thumbnailer"), filepath.Join(t.TempDir(), Program)} {
		if _, err := Locate(path); err == nil {
			t.Errorf("Locate(%s) found a helper", path)
		}
	}
}
