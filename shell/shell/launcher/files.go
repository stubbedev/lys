package launcher

import (
	"fmt"
	"log"
	"os"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/shell/service/fileindex"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
	"github.com/stubbedev/lys/shell/service/launcher/search"
)

// fileIndex is the index the files mode searches: opened by the first
// session that asks for it and kept for the shell's lifetime, so typing
// reads memory, and opened again once the roots it covers change. It is
// touched only on the UI loop.
type fileIndex struct {
	open  func(fileindex.Config) (*fileindex.Index, error)
	key   string
	index *fileindex.Index
}

// get is the index over cfg's roots; nil when it cannot be opened, which
// is logged once for those roots.
func (f *fileIndex) get(cfg config.LauncherSearchConfig) modes.FileIndex {
	ic, err := search.IndexConfig(cfg, os.Getenv("HOME"))
	key := fmt.Sprintf("%q\x00%q\x00%q", ic.Roots, ic.Rules.Exclude, ic.RecentPath)
	if key == f.key {
		if f.index == nil {
			return nil
		}
		return f.index
	}
	f.close()
	f.key = key
	if err == nil {
		f.index, err = f.open(ic)
	}
	if err != nil {
		log.Printf("launcher: no file search: %v", err)
		return nil
	}
	return f.index
}

func (f *fileIndex) close() {
	if f.index != nil {
		f.index.Close()
		f.index = nil
	}
}
