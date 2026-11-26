// Command lys-docthumb writes the first page of a document as a PNG
// thumbnail, for the thumbnailer that runs it as a child process of its
// own, or, run with dochelper.ServeCommand, serves one document to a
// reader over the protocol of dochelper on its standard input and
// output. It parses untrusted input with folio, so it caps its own
// memory before it opens the file and leaves its deadlines to the
// program running it, which kills its process group whole. Its command
// lines, protocol, memory caps and exit statuses are dochelper's.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/docrender"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// run serves the document or makes the thumbnail args ask for and is
// the status to exit with, having said on stderr what went wrong. A
// page drawn in part is still a thumbnail, and what was left out of it
// is said too.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if s, ok, err := dochelper.ParseServing(args); ok {
		if err == nil {
			err = serve(s, stdin, stdout)
		}
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
		}
		return dochelper.ExitCode(err)
	}
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
	if err := dochelper.LimitMemory(req.Memory); err != nil {
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

// serve serves one document as s says, reading requests from stdin and
// answering on stdout until it is told to close or stdin ends.
func serve(s dochelper.Serving, stdin io.Reader, stdout io.Writer) error {
	if err := s.Memory.Limit(); err != nil {
		return err
	}
	docrender.AddFontDirs(s.FontDirs)
	return dochelper.Serve(stdin, stdout, &docrender.Reader{})
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
