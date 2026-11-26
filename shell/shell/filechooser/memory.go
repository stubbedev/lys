package filechooser

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/stubbedev/lys/desktop/fileuri"

	"github.com/stubbedev/lys/shell/internal/statefile"
)

// maxAppFolders is how many apps' last folders are kept.
const maxAppFolders = 64

// folderMemory is the folder each app last picked in, the most recent
// first, kept in a file of "app id<TAB>folder" lines. It is read and
// written under its lock, from the loop and from the requests' own
// goroutines.
type folderMemory struct {
	mu   sync.Mutex
	apps []appFolder
	file statefile.File
}

type appFolder struct{ app, dir string }

// loadFolderMemory reads the memory kept in path; a file that is not
// there is an empty memory.
func loadFolderMemory(path string) *folderMemory {
	m := &folderMemory{file: statefile.File{Path: path}}
	if path == "" {
		return m
	}
	data, err := os.ReadFile(path) //nolint:gosec // the chooser's own state file
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("file chooser: reading %s: %v", path, err)
		}
		return m
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() && len(m.apps) < maxAppFolders {
		app, dir, ok := strings.Cut(sc.Text(), "\t")
		if ok && app != "" && filepath.IsAbs(dir) {
			m.apps = append(m.apps, appFolder{app, dir})
		}
	}
	return m
}

// folder is where app last picked, "" for an app never seen.
func (m *folderMemory) folder(app string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.apps {
		if a.app == app {
			return a.dir
		}
	}
	return ""
}

// remember makes dir app's last folder and saves the memory, the
// least recent app dropped past maxAppFolders.
func (m *folderMemory) remember(app, dir string) {
	m.mu.Lock()
	apps := make([]appFolder, 0, maxAppFolders)
	apps = append(apps, appFolder{app, dir})
	for _, a := range m.apps {
		if a.app != app && len(apps) < maxAppFolders {
			apps = append(apps, a)
		}
	}
	m.apps = apps
	var b strings.Builder
	for _, a := range apps {
		b.WriteString(a.app + "\t" + a.dir + "\n")
	}
	m.mu.Unlock()
	m.file.Write([]byte(b.String()))
}

// foldersFile is where the folder memory is kept beside the view
// file, "" for none.
func foldersFile(viewFile string) string {
	if viewFile == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(viewFile), "file-chooser-folders")
}

// rememberFolder keeps where the app showing picked uri for its next
// request: the folder picked, or the folder of the file.
func (c *Chooser) rememberFolder(uri string) {
	path, ok := fileuri.Path(uri)
	if c.caller.AppID == "" || !ok {
		return
	}
	if !c.mode.folders() {
		path = filepath.Dir(path)
	}
	c.folders.remember(c.caller.AppID, path)
}
