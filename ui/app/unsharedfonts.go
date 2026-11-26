package app

import (
	"errors"
	"fmt"
	"sync"

	"github.com/stubbedev/lys/lib/assetfile"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
	"github.com/stubbedev/lys/lib/third_party/typesetting/fontscan"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/ui/internal/sysfont"
	"github.com/stubbedev/lys/ui/internal/uifont"
)

const unsharedFontLimit = 512 << 20

// UnsharedFonts is a typeface of each face screen text is most often
// drawn in: Geist regular and bold and Geist Mono, the faces the default
// look is drawn in, then the faces sans-serif, serif and monospace
// resolve to, each once. None is shared with the app or with another
// call, so a goroutine off the UI loop may use them, one goroutine at a
// time. A generic family that resolves to no face is an error.
func UnsharedFonts() ([]*render.Typeface, error) {
	geist, err := unsharedFonts.embedded(uifont.Family, uifont.Sans)
	if err != nil {
		return nil, err
	}
	mono, err := unsharedFonts.embedded(uifont.MonoFamily, uifont.Mono)
	if err != nil {
		return nil, err
	}
	out := []*render.Typeface{geist, geist.Weighted(700), mono}
	seen := map[string]bool{
		font.NormalizeFamily(uifont.Family):     true,
		font.NormalizeFamily(uifont.MonoFamily): true,
	}
	for _, generic := range []struct {
		name    string
		resolve func() (*render.Typeface, error)
	}{
		{fontscan.SansSerif, sysfont.Sans},
		{fontscan.Serif, sysfont.Serif},
		{fontscan.Monospace, sysfont.Monospace},
	} {
		shared, err := generic.resolve()
		if err != nil {
			return nil, fmt.Errorf("app: resolve the %s face: %w", generic.name, err)
		}
		family := shared.Describe().Family
		key := font.NormalizeFamily(family)
		if seen[key] {
			continue
		}
		seen[key] = true
		loc, err := sysfont.RegularFace(family)
		if err != nil {
			return nil, fmt.Errorf("app: locate %s, the %s face: %w", family, generic.name, err)
		}
		tf, err := unsharedFonts.installed(loc)
		if err != nil {
			return nil, fmt.Errorf("app: load %s, the %s face: %w", family, generic.name, err)
		}
		out = append(out, tf)
	}
	return out, nil
}

var unsharedFonts = fontFiles{parsed: map[fontscan.Location]*font.Font{}}

type fontFiles struct {
	mu     sync.Mutex
	parsed map[fontscan.Location]*font.Font
}

func (f *fontFiles) embedded(family string, data []byte) (*render.Typeface, error) {
	return f.typeface(fontscan.Location{File: "embedded:" + family}, func() ([]byte, func() error, error) {
		return data, func() error { return nil }, nil
	})
}

func (f *fontFiles) installed(loc fontscan.Location) (*render.Typeface, error) {
	return f.typeface(loc, func() ([]byte, func() error, error) {
		data, err := mapFontFile(loc.File)
		if err != nil {
			return nil, nil, err
		}
		return data, func() error { return unix.Munmap(data) }, nil
	})
}

func (f *fontFiles) typeface(loc fontscan.Location, load func() (data []byte, release func() error, err error)) (*render.Typeface, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ft, ok := f.parsed[loc]
	if !ok {
		data, release, err := load()
		if err != nil {
			return nil, err
		}
		if ft, err = parseFont(data, loc); err != nil {
			return nil, errors.Join(err, release())
		}
		f.parsed[loc] = ft
	}
	return render.NewTypeface(font.NewFace(ft))
}

func parseFont(data []byte, loc fontscan.Location) (*font.Font, error) {
	loaders, err := opentype.NewLoaders(opentype.NewMapped(data))
	if err != nil {
		return nil, fmt.Errorf("app: read %s: %w", loc.File, err)
	}
	if int(loc.Index) >= len(loaders) {
		return nil, fmt.Errorf("app: %s has no face %d", loc.File, loc.Index)
	}
	ft, err := font.NewFont(loaders[loc.Index])
	if err != nil {
		return nil, fmt.Errorf("app: face %d of %s: %w", loc.Index, loc.File, err)
	}
	return ft, nil
}

func mapFontFile(path string) ([]byte, error) {
	f, size, err := assetfile.Open(path, unsharedFontLimit)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := unix.Mmap(int(f.Fd()), 0, int(size), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		return nil, fmt.Errorf("app: map %s: %w", path, err)
	}
	return data, nil
}
