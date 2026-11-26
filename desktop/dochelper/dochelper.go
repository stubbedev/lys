// Package dochelper is the contract between lys and lys-docthumb, the
// program that renders documents in a sandboxed process of its own so
// that no long-running lys process parses them: its command line, the
// exit statuses its failures are told by, and the document types it
// thumbnails. It links no renderer.
package dochelper

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
)

// Program is the helper's executable, installed next to the programs
// that run it.
const Program = "lys-docthumb"

// The failures a document renders with.
var (
	ErrDamaged     = errors.New("document: damaged")
	ErrEncrypted   = errors.New("document: encrypted with a password")
	ErrTooLarge    = errors.New("document: too large to render")
	ErrUnsupported = errors.New("document: unsupported")
)

// The helper's exit statuses past success. exitFailed is a failure none
// of the errors names, and exitRuntime the status the Go runtime exits
// with on a fatal error, which running out of the helper's memory is.
const (
	exitFailed  = 1
	exitRuntime = 2
)

// exits are the statuses the errors are told by.
var exits = []struct {
	code int
	err  error
}{
	{3, ErrDamaged},
	{4, ErrEncrypted},
	{5, ErrTooLarge},
	{6, ErrUnsupported},
}

// ExitCode is the status the helper exits with after err: 0 for none.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	for _, e := range exits {
		if errors.Is(err, e.err) {
			return e.code
		}
	}
	return exitFailed
}

// The Go runtime's fatal error line, which when the helper's memory
// limit fails an allocation says it ran out of memory (in a few
// wordings, by where the allocation was).
var (
	fatalError  = []byte("fatal error: ")
	outOfMemory = []byte("out of memory")
)

// ExitError is the error a helper's exit status and output tell: one of
// the errors for the statuses ExitCode gives them, ErrTooLarge for the
// runtime dying out of memory, and nil for any other status.
func ExitError(code int, output []byte) error {
	if code == exitRuntime && ranOutOfMemory(output) {
		return ErrTooLarge
	}
	for _, e := range exits {
		if e.code == code {
			return e.err
		}
	}
	return nil
}

// ranOutOfMemory reports output holding the runtime's fatal error for
// running out of memory.
func ranOutOfMemory(output []byte) bool {
	for line := range bytes.Lines(output) {
		if fatal, ok := bytes.CutPrefix(line, fatalError); ok && bytes.Contains(fatal, outOfMemory) {
			return true
		}
	}
	return false
}

// MaxSize is the largest box side a request may ask for.
const MaxSize = 4096

// Request is one thumbnail: the first page of the document at Input,
// fitted into a Size square and written to Output as a PNG, by a helper
// holding at most Memory bytes and looking up the fonts the document
// does not carry in FontDirs.
type Request struct {
	Input, Output string
	Size          int
	Memory        int64
	FontDirs      []string
}

const (
	sizeFlag    = "size"
	memoryFlag  = "memory"
	fontDirFlag = "font-dir"
)

// Args is the helper's command line for r, after the program.
func (r Request) Args() []string {
	args := make([]string, 0, 6+2*len(r.FontDirs))
	args = append(args, "-"+sizeFlag, strconv.Itoa(r.Size), "-"+memoryFlag, strconv.FormatInt(r.Memory, 10))
	for _, dir := range r.FontDirs {
		args = append(args, "-"+fontDirFlag, dir)
	}
	return append(args, "--", r.Input, r.Output)
}

// ParseRequest reads the request Args wrote.
func ParseRequest(args []string) (Request, error) {
	var r Request
	fs := flag.NewFlagSet(Program, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&r.Size, sizeFlag, 0, "")
	fs.Int64Var(&r.Memory, memoryFlag, 0, "")
	fs.Func(fontDirFlag, "", func(dir string) error {
		r.FontDirs = append(r.FontDirs, dir)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return Request{}, fmt.Errorf("%s: %w", Program, err)
	}
	if fs.NArg() != 2 {
		return Request{}, fmt.Errorf("%s: want an input and an output, got %q", Program, fs.Args())
	}
	r.Input, r.Output = fs.Arg(0), fs.Arg(1)
	if r.Input == "" || r.Output == "" {
		return Request{}, fmt.Errorf("%s: an empty input or output path", Program)
	}
	if r.Size < 1 || r.Size > MaxSize {
		return Request{}, fmt.Errorf("%s: size %d is not within 1 to %d", Program, r.Size, MaxSize)
	}
	if r.Memory <= 0 {
		return Request{}, fmt.Errorf("%s: memory %d is not positive", Program, r.Memory)
	}
	return r, nil
}

// types are the MIME types the helper thumbnails, as shared-mime-info
// names them.
var types = []string{
	"application/pdf",
	"application/epub+zip",
	"application/x-mobipocket-ebook",
	"application/vnd.amazon.mobi8-ebook",
	"application/x-fictionbook+xml",
	"application/vnd.ms-htmlhelp",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
}

// Thumbnails reports whether the helper thumbnails files of mimeType.
func Thumbnails(mimeType string) bool { return slices.Contains(types, mimeType) }
