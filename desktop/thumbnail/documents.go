package thumbnail

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/lib/third_party/typesetting/fontscan"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// DocumentMemory is the most memory lys-docthumb may hold making one
// thumbnail, with room above the 583 MiB the costliest page measured
// (two 14-megapixel JPEG 2000 layers) peaks at. Past it the helper dies
// and the thumbnail is dochelper.ErrTooLarge.
const DocumentMemory = 768 << 20

// sandboxFonts is where the font directories the sandbox's system does
// not hold are read in it.
const sandboxFonts = "/run/lys-fonts"

// documentHelper is lys-docthumb as what runs, and the sandbox it runs
// in.
type documentHelper struct {
	sandbox sandbox
	maker   maker
}

// helperArgv is the command line lys-docthumb runs by: argv when set,
// else the program of that name next to the running one.
func helperArgv(argv []string) ([]string, error) {
	if len(argv) > 0 {
		return argv, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("finding the running program: %w", err)
	}
	return []string{filepath.Join(filepath.Dir(exe), dochelper.Program)}, nil
}

// loadDocumentHelper is lys-docthumb run by argv (helperArgv's) in sb,
// reading the system's font directories, or why there is none.
func loadDocumentHelper(sb sandbox, argv []string) (documentHelper, error) {
	argv, err := helperArgv(argv)
	if err != nil {
		return documentHelper{}, err
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return documentHelper{}, err
	}
	binds, dirs := fontBinds(sb, fontDirs())
	prefix := slices.Clip(slices.Clone(argv))
	m := maker{
		name: argv[0],
		argv: func(input, _, output string, size int) []string {
			req := dochelper.Request{Input: input, Output: output, Size: size, Memory: DocumentMemory, FontDirs: dirs}
			return append(prefix, req.Args()...)
		},
		binds: binds,
		exit:  dochelper.ExitError,
	}
	return documentHelper{sandbox: sb, maker: m}, nil
}

// fontDirs are the directories lys finds fonts in: fontconfig's, the
// XDG data directories' and the usual system ones that exist.
func fontDirs() []string {
	dirs, err := fontscan.DefaultFontDirectories(log.Default())
	if err != nil {
		log.Printf("thumbnail: documents are drawn with the fonts they carry and folio's own: %v", err)
	}
	return dirs
}

// fontBinds are the binds making dirs readable in sb and the paths
// they are read at there: a directory the sandbox's system holds where
// it is, any other under sandboxFonts.
func fontBinds(sb sandbox, dirs []string) ([]bind, []string) {
	var binds []bind
	at := make([]string, 0, len(dirs))
	for i, dir := range dirs {
		if sb.visible(dir) {
			at = append(at, dir)
			continue
		}
		b := bind{host: dir, at: path.Join(sandboxFonts, strconv.Itoa(i))}
		binds = append(binds, b)
		at = append(at, b.at)
	}
	return binds, at
}
