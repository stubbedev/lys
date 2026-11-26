// Package dochelper is the contract between lys and lys-docthumb, the
// program that renders documents in a process of its own so that no
// long-running lys process parses them: where it is installed and how
// it is started, its command lines (one thumbnail, or serving one open
// document over the framed protocol of protocol.go), the memory cap it
// holds itself to, the exit statuses its failures are told by, and the
// document types it reads. It links no renderer.
package dochelper

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/lib/intconv"
	"golang.org/x/sys/unix"
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
	ErrTimeout     = errors.New("document: took too long to render")
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
// limit fails an allocation says it ran out of memory or could not
// allocate it (in a few wordings, by where the allocation was).
var (
	fatalError  = []byte("fatal error: ")
	outOfMemory = [][]byte{[]byte("out of memory"), []byte("cannot allocate memory")}
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
		fatal, ok := bytes.CutPrefix(line, fatalError)
		if ok && slices.ContainsFunc(outOfMemory, func(w []byte) bool { return bytes.Contains(fatal, w) }) {
			return true
		}
	}
	return false
}

// softShare is the part of the hard memory limit the garbage collector
// keeps the heap within when no soft limit is given.
const softShare = 3

// Memory is what a helper may hold: Hard bytes of mapped data, past
// which an allocation kills it, and Soft bytes of heap the collector
// works to stay within.
type Memory struct {
	Hard, Soft int64
}

// LimitMemory caps the data the calling process maps at limit bytes
// and has the collector work to stay within a third of it, as Memory's
// Limit does.
func LimitMemory(limit int64) error { return Memory{Hard: limit, Soft: limit / softShare}.Limit() }

// Limit caps the data the calling process maps at m.Hard bytes, past
// which an allocation kills it with the runtime's out of memory error
// (ErrTooLarge to ExitError), and has the collector work to stay within
// m.Soft. RLIMIT_AS would count the address space the Go runtime
// reserves without using, which is most of a small limit.
func (m Memory) Limit() error {
	if err := m.validate(); err != nil {
		return err
	}
	lim, err := intconv.Exact[uint64](m.Hard)
	if err != nil {
		return fmt.Errorf("%s: a memory limit of %d bytes: %w", Program, m.Hard, err)
	}
	if err := unix.Setrlimit(unix.RLIMIT_DATA, &unix.Rlimit{Cur: lim, Max: lim}); err != nil {
		return fmt.Errorf("%s: limiting its memory to %d bytes: %w", Program, m.Hard, err)
	}
	debug.SetMemoryLimit(m.Soft)
	return nil
}

func (m Memory) validate() error {
	if m.Hard <= 0 || m.Soft <= 0 || m.Soft > m.Hard {
		return fmt.Errorf("%s: memory of %d soft and %d hard bytes is not positive with soft within hard", Program, m.Soft, m.Hard)
	}
	return nil
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
	args = appendFontDirs(args, r.FontDirs)
	return append(args, "--", r.Input, r.Output)
}

// ParseRequest reads the request Args wrote.
func ParseRequest(args []string) (Request, error) {
	var r Request
	fs := flag.NewFlagSet(Program, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.IntVar(&r.Size, sizeFlag, 0, "")
	fs.Int64Var(&r.Memory, memoryFlag, 0, "")
	fontDirsFlag(fs, &r.FontDirs)
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

// appendFontDirs is args with a font directory flag for each of dirs.
func appendFontDirs(args, dirs []string) []string {
	for _, dir := range dirs {
		args = append(args, "-"+fontDirFlag, dir)
	}
	return args
}

// fontDirsFlag reads every font directory flag into dirs, in order.
func fontDirsFlag(fs *flag.FlagSet, dirs *[]string) {
	fs.Func(fontDirFlag, "", func(dir string) error {
		*dirs = append(*dirs, dir)
		return nil
	})
}

// ServeCommand is the first argument of a helper serving one document
// over its standard input and output (Serve) until it is told to close
// or its input ends.
const ServeCommand = "serve"

// Serving is the helper serving one document, holding Memory and
// looking up the fonts the document does not carry in FontDirs.
type Serving struct {
	Memory   Memory
	FontDirs []string
}

const softMemoryFlag = "soft-memory"

// Args is the helper's command line for s, after the program.
func (s Serving) Args() []string {
	args := make([]string, 0, 5+2*len(s.FontDirs))
	args = append(args, ServeCommand, "-"+memoryFlag, strconv.FormatInt(s.Memory.Hard, 10), "-"+softMemoryFlag, strconv.FormatInt(s.Memory.Soft, 10))
	return appendFontDirs(args, s.FontDirs)
}

// ParseServing reads the command line Serving's Args wrote, and is
// false for one that does not start with ServeCommand.
func ParseServing(args []string) (Serving, bool, error) {
	if len(args) == 0 || args[0] != ServeCommand {
		return Serving{}, false, nil
	}
	var s Serving
	fs := flag.NewFlagSet(Program+" "+ServeCommand, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int64Var(&s.Memory.Hard, memoryFlag, 0, "")
	fs.Int64Var(&s.Memory.Soft, softMemoryFlag, 0, "")
	fontDirsFlag(fs, &s.FontDirs)
	if err := fs.Parse(args[1:]); err != nil {
		return Serving{}, true, fmt.Errorf("%s: %w", Program, err)
	}
	if fs.NArg() != 0 {
		return Serving{}, true, fmt.Errorf("%s %s: unexpected arguments %q", Program, ServeCommand, fs.Args())
	}
	if err := s.Memory.validate(); err != nil {
		return Serving{}, true, err
	}
	return s, true, nil
}

// types are the MIME types the helper reads, as shared-mime-info names
// them.
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

// Thumbnails reports whether the helper thumbnails and reads files of
// mimeType.
func Thumbnails(mimeType string) bool { return slices.Contains(types, mimeType) }

// Types are the MIME types the helper reads.
func Types() []string { return slices.Clone(types) }
