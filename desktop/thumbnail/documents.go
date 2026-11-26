package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// DocumentMemory is the most memory lys-docthumb may hold making one
// thumbnail, with room above the 583 MiB the costliest page measured
// (two 14-megapixel JPEG 2000 layers) peaks at. Past it the helper dies
// and the thumbnail is dochelper.ErrTooLarge.
const DocumentMemory = 768 << 20

// DefaultRunTimeout is how long lys-docthumb may run before it counts
// as failed, and helperMaxBytes how large its PNG may be.
const (
	DefaultRunTimeout = dochelper.RequestTimeout
	helperMaxBytes    = 16 << 20
)

// outputName is the file lys-docthumb writes in its own temporary
// directory.
const outputName = "thumbnail.png"

// docHelper is lys-docthumb, the one program package thumbnail runs:
// the executable called dochelper.Program, and the font directories it
// looks up the fonts a document does not carry in.
type docHelper struct {
	path     string
	fontDirs []string
}

// newDocHelper is lys-docthumb at path, as dochelper.Locate finds it,
// reading the system's font directories.
func newDocHelper(path string) (docHelper, error) {
	path, err := dochelper.Locate(path)
	if err != nil {
		return docHelper{}, err
	}
	return docHelper{path: path, fontDirs: dochelper.FontDirs()}, nil
}

// runError is lys-docthumb's failure and the start of what it printed.
type runError struct {
	err    error
	output []byte
}

func (e *runError) Error() string {
	return fmt.Sprintf("%v: %s", e.err, strings.TrimSpace(string(e.output)))
}

func (e *runError) Unwrap() error { return e.err }

// thumbnail runs lys-docthumb on input at size, as a child process with
// an empty environment, in a process group of its own killed whole at
// timeout or once ctx ends, writing into a temporary directory of its
// own, and decodes its PNG within the budget. A helper that fails is
// the error its exit status tells (dochelper.ExitError), wrapping what
// it printed; one that writes nothing, more than helperMaxBytes or an
// image that does not decode fails too.
func (h docHelper) thumbnail(ctx context.Context, c *core, input string, size int) (image.Image, error) {
	dir, err := os.MkdirTemp("", "lys-docthumb-")
	if err != nil {
		return nil, fmt.Errorf("%s on %s: %w", dochelper.Program, input, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	output := filepath.Join(dir, outputName)
	req := dochelper.Request{Input: input, Output: output, Size: size, Memory: DocumentMemory, FontDirs: h.fontDirs}
	if err := h.run(ctx, req, c.runTimeout); err != nil {
		return nil, fmt.Errorf("%s on %s: %w", dochelper.Program, input, err)
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, fmt.Errorf("%s on %s wrote nothing: %w", dochelper.Program, input, err)
	}
	if info.Size() > helperMaxBytes {
		return nil, fmt.Errorf("%s on %s wrote %d bytes, more than %d", dochelper.Program, input, info.Size(), helperMaxBytes)
	}
	img, err := c.decodeWithin(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("%s on %s: %w", dochelper.Program, input, err)
	}
	return img, nil
}

// run runs lys-docthumb on req, started as dochelper.Command starts
// it, until it exits, timeout passes or ctx
// ends; a failure is a *runError wrapped in the error its exit status
// tells, where it tells one.
func (h docHelper) run(ctx context.Context, req dochelper.Request, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out := &dochelper.Head{Limit: dochelper.OutputBytes}
	cmd := dochelper.Command(ctx, h.path, req.Args())
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		err = fmt.Errorf("%w (%w)", err, ctx.Err())
	}
	failed := &runError{err: err, output: out.Bytes()}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		if told := dochelper.ExitError(exit.ExitCode(), out.Bytes()); told != nil {
			return fmt.Errorf("%w: %w", told, failed)
		}
	}
	return failed
}
