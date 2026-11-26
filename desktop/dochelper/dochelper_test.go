package dochelper

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// Every error the helper exits after is told again by its status, and a
// status no error has tells none.
func TestExitStatusesTellTheErrors(t *testing.T) {
	for _, want := range []error{ErrDamaged, ErrEncrypted, ErrTooLarge, ErrUnsupported} {
		code := ExitCode(fmt.Errorf("opening x: %w", want))
		if code <= exitRuntime {
			t.Errorf("%v exits %d, a status of its own is past %d", want, code, exitRuntime)
		}
		if got := ExitError(code, nil); !errors.Is(got, want) {
			t.Errorf("status %d tells %v, want %v", code, got, want)
		}
	}
	if code := ExitCode(nil); code != 0 {
		t.Errorf("success exits %d", code)
	}
	if code := ExitCode(errors.New("disk full")); code != exitFailed {
		t.Errorf("an untyped failure exits %d, want %d", code, exitFailed)
	}
	for _, code := range []int{0, exitFailed, exitRuntime, 99, -1} {
		if err := ExitError(code, []byte("panic: boom")); err != nil {
			t.Errorf("status %d tells %v, want none", code, err)
		}
	}
}

// The Go runtime dying out of memory, as the helper's limit makes it, is
// ErrTooLarge; another status printing the same is not.
func TestRunningOutOfMemoryIsTooLarge(t *testing.T) {
	for _, fatal := range []string{"fatal error: out of memory", "fatal error: runtime: out of memory", "fatal error: out of memory allocating heap arena metadata"} {
		out := []byte("runtime: out of memory: cannot allocate 100663296-byte block (3866624 in use)\n" + fatal + "\n\ngoroutine 1 [running]:\n")
		if err := ExitError(exitRuntime, out); !errors.Is(err, ErrTooLarge) {
			t.Errorf("the runtime dying with %q = %v, want ErrTooLarge", fatal, err)
		}
		if err := ExitError(exitFailed, out); err != nil {
			t.Errorf("status %d printing %q = %v, want none", exitFailed, fatal, err)
		}
	}
	for _, other := range []string{"panic: out of memory in my parser", "fatal error: concurrent map writes"} {
		if err := ExitError(exitRuntime, []byte(other+"\n")); err != nil {
			t.Errorf("the runtime dying with %q = %v, want none", other, err)
		}
	}
}

// A request reads back as written, font directories in order, and an
// input or output that looks like a flag stays a path.
func TestARequestRoundTrips(t *testing.T) {
	want := Request{Input: "-in.pdf", Output: "/tmp/out.png", Size: 256, Memory: 768 << 20, FontDirs: []string{"/usr/share/fonts", "/run/lys-fonts/1"}}
	got, err := ParseRequest(want.Args())
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseRequest(%q) = %+v, %v, want %+v", want.Args(), got, err, want)
	}
}

// A request without its paths, or with a size or memory out of range,
// is refused.
func TestABadRequestIsRefused(t *testing.T) {
	good := Request{Input: "in", Output: "out", Size: 128, Memory: 1 << 20}
	for name, r := range map[string]Request{
		"no input":    {Output: "out", Size: 128, Memory: 1 << 20},
		"zero size":   {Input: "in", Output: "out", Memory: 1 << 20},
		"huge size":   {Input: "in", Output: "out", Size: MaxSize + 1, Memory: 1 << 20},
		"no memory":   {Input: "in", Output: "out", Size: 128},
		"neg. memory": {Input: "in", Output: "out", Size: 128, Memory: -1},
	} {
		if _, err := ParseRequest(r.Args()); err == nil {
			t.Errorf("%s: %q parsed", name, r.Args())
		}
	}
	for name, args := range map[string][]string{
		"one path":   good.Args()[:len(good.Args())-1],
		"three":      append(good.Args(), "more"),
		"bad flag":   append([]string{"-colour", "red"}, good.Args()...),
		"bad number": {"-size", "big", "-memory", "1", "--", "in", "out"},
	} {
		if _, err := ParseRequest(args); err == nil {
			t.Errorf("%s: %q parsed", name, args)
		}
	}
}

// PDFs, books and office documents are the helper's; images and text
// are not.
func TestTheHelperThumbnailsDocuments(t *testing.T) {
	for _, mimeType := range []string{"application/pdf", "application/epub+zip", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"} {
		if !Thumbnails(mimeType) {
			t.Errorf("%s is not thumbnailed", mimeType)
		}
	}
	for _, mimeType := range []string{"image/png", "image/svg+xml", "text/plain", "text/html", ""} {
		if Thumbnails(mimeType) {
			t.Errorf("%s is thumbnailed", mimeType)
		}
	}
}
