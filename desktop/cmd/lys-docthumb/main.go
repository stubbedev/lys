// Command lys-docthumb writes the first page of a document as a PNG
// thumbnail, for the thumbnailer that runs it in its sandbox. It parses
// untrusted input with folio, so it caps its own memory before it opens
// the file and leaves its deadline to the thumbnailer, which kills it
// whole. Its command line and exit statuses are dochelper's.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"runtime/debug"

	"github.com/stubbedev/lys/lib/intconv"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/docrender"
)

func main() { os.Exit(run(os.Args[1:], os.Stderr)) }

// run makes the thumbnail args ask for and is the status to exit with,
// having said on stderr what went wrong. A page drawn in part is still
// a thumbnail, and what was left out of it is said too.
func run(args []string, stderr io.Writer) int {
	left, err := makeThumbnail(args)
	for _, e := range []error{left, err} {
		if e != nil {
			_, _ = fmt.Fprintln(stderr, e)
		}
	}
	return dochelper.ExitCode(err)
}

// makeThumbnail writes the thumbnail args ask for, or fails with err; left
// is what a page drawn in part left out.
func makeThumbnail(args []string) (left, err error) {
	req, err := dochelper.ParseRequest(args)
	if err != nil {
		return nil, err
	}
	if err := limitMemory(req.Memory); err != nil {
		return nil, err
	}
	docrender.AddFontDirs(req.FontDirs)
	doc, err := docrender.Open(req.Input)
	if err != nil {
		return nil, err
	}
	img, left := doc.Render(0, req.Size, req.Size)
	if err := doc.Close(); err != nil {
		return nil, errors.Join(left, err)
	}
	if img == nil {
		return nil, left
	}
	if err := writePNG(req.Output, img); err != nil {
		return nil, err
	}
	return left, nil
}

// softShare is the part of the hard memory limit the garbage collector
// keeps the heap within.
const softShare = 3

// limitMemory caps the data the process maps at limit bytes, past which
// an allocation kills it, and has the collector work to stay within a
// third of it. RLIMIT_AS would count the address space the Go runtime
// reserves without using, which is most of a small limit.
func limitMemory(limit int64) error {
	lim, err := intconv.Exact[uint64](limit)
	if err != nil {
		return fmt.Errorf("%s: a memory limit of %d bytes: %w", dochelper.Program, limit, err)
	}
	if err := unix.Setrlimit(unix.RLIMIT_DATA, &unix.Rlimit{Cur: lim, Max: lim}); err != nil {
		return fmt.Errorf("%s: limiting its memory to %d bytes: %w", dochelper.Program, limit, err)
	}
	debug.SetMemoryLimit(limit / softShare)
	return nil
}

// writePNG writes img to path as a PNG.
func writePNG(path string, img image.Image) error {
	f, err := os.Create(path) //nolint:gosec // the output its command line names
	if err != nil {
		return fmt.Errorf("%s: %w", dochelper.Program, err)
	}
	w := bufio.NewWriter(f)
	err = (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(w, img)
	if err == nil {
		err = w.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("%s: writing %s: %w", dochelper.Program, path, err)
	}
	return nil
}
