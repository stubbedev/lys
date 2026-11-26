package thumbnail

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/shlex"
)

// DefaultRunTimeout is how long a thumbnailer may run before it counts
// as failed, and thumbnailerMaxBytes how large its PNG may be.
const (
	DefaultRunTimeout   = 10 * time.Second
	thumbnailerMaxBytes = 16 << 20
)

// thumbnailerGroup is a .thumbnailer file's group.
const thumbnailerGroup = "Thumbnailer Entry"

// outputName is the file a thumbnailer writes in its sandbox's
// writable directory.
const outputName = "thumbnail.png"

// Command is a thumbnailer's command line for an input read at input
// and an output written to output, both paths inside the sandbox, at
// size pixels; an empty one is a command that does not parse.
type Command func(input, output string, size int) []string

// maker is what runs to make a thumbnail: its command line for the
// sandbox's input path and URI, output path and size, and what it is
// called in errors (the .thumbnailer file, or the command's source).
type maker struct {
	name string
	argv func(input, uri, output string, size int) []string
}

// commandMaker is cmd as a maker named name.
func commandMaker(name string, cmd Command) maker {
	return maker{name: name, argv: func(input, _, output string, size int) []string { return cmd(input, output, size) }}
}

// thumbnailerEntry is an installed thumbnailer: the command line its
// Exec names, with the %i %o %s %u field codes still in it, and the
// file that installed it.
type thumbnailerEntry struct {
	argv []string
	file string
}

// loadThumbnailers reads the thumbnailers/*.thumbnailer files of the
// data directories into a thumbnailer per MIME type: an earlier
// directory wins, as does the earlier file within one, and a
// thumbnailer whose TryExec is not installed or whose Exec does not
// parse is left out.
func loadThumbnailers(dataDirs []string) map[string]thumbnailerEntry {
	byType := map[string]thumbnailerEntry{}
	for _, dir := range dataDirs {
		files, err := filepath.Glob(filepath.Join(dir, "thumbnailers", "*.thumbnailer"))
		if err != nil {
			continue
		}
		slices.Sort(files)
		for _, file := range files {
			kf, err := desktopentry.LoadKeyFile(file)
			if err != nil || !kf.HasGroup(thumbnailerGroup) {
				continue
			}
			if try, ok := kf.String(thumbnailerGroup, "TryExec"); ok {
				if _, err := exec.LookPath(try); err != nil {
					continue
				}
			}
			line, _ := kf.String(thumbnailerGroup, "Exec")
			argv, ok := shlex.Split(line)
			if !ok || len(argv) == 0 {
				continue
			}
			for _, t := range kf.List(thumbnailerGroup, "MimeType") {
				if _, taken := byType[t]; !taken {
					byType[t] = thumbnailerEntry{argv: argv, file: file}
				}
			}
		}
	}
	return byType
}

// expand fills the command line's field codes: %i the input path, %u
// its URI, %o the output path, %s the size and %% a percent sign.
func (t thumbnailerEntry) expand(input, uri, output string, size int) []string {
	r := strings.NewReplacer("%%", "%", "%i", input, "%u", uri, "%o", output, "%s", strconv.Itoa(size))
	argv := make([]string, len(t.argv))
	for i, a := range t.argv {
		argv[i] = r.Replace(a)
	}
	return argv
}

// maker is the installed thumbnailer as what runs.
func (t thumbnailerEntry) maker() maker { return maker{name: t.file, argv: t.expand} }

// run makes input's thumbnail at size with m, in sb: it reads the input
// under its sandbox path and URI and writes into a temporary directory
// of its own, and is killed at the core's run timeout or when ctx ends.
// A command that does not parse, fails, writes nothing, or writes more
// than thumbnailerMaxBytes or an image that does not decode within the
// budget, fails.
func (c *core) run(ctx context.Context, sb sandbox, m maker, input string, size int) (image.Image, error) {
	argv := m.argv(sandboxPath(input), sandboxURI(input), path.Join(sandboxDir, outputName), size)
	if len(argv) == 0 {
		return nil, fmt.Errorf("thumbnailer %s: the command does not parse", m.name)
	}
	dir, err := os.MkdirTemp("", "lys-thumbnailer-")
	if err != nil {
		return nil, fmt.Errorf("thumbnailer %s: %w", m.name, err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := sb.run(ctx, input, dir, argv, c.runTimeout); err != nil {
		return nil, fmt.Errorf("thumbnailer %s on %s: %w", m.name, input, err)
	}
	output := filepath.Join(dir, outputName)
	info, err := os.Stat(output)
	if err != nil {
		return nil, fmt.Errorf("thumbnailer %s on %s wrote nothing: %w", m.name, input, err)
	}
	if info.Size() > thumbnailerMaxBytes {
		return nil, fmt.Errorf("thumbnailer %s on %s wrote %d bytes, more than %d", m.name, input, info.Size(), thumbnailerMaxBytes)
	}
	img, err := c.decodeWithin(ctx, output)
	if err != nil {
		return nil, fmt.Errorf("thumbnailer %s on %s: %w", m.name, input, err)
	}
	return img, nil
}
