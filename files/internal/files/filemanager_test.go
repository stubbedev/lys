package files

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/transfer"

	"github.com/stubbedev/lys/files/internal/filemanager1"
)

// serve exports FileManager1 for hs on a private bus and returns a
// caller's view of it.
func (hs *harness) serve(t *testing.T) dbus.BusObject {
	t.Helper()
	bus := dbustest.Start(t)
	release, err := filemanager1.NewDaemon(fileManager{show: func(do func(*Files), _ string) {
		hs.Invoke(func() { do(hs.f) })
	}}).Export(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return bus.Conn(t).Object(filemanager1.ServiceName, filemanager1.ServicePath)
}

func (hs *harness) call(t *testing.T, obj dbus.BusObject, method string, paths ...string) error {
	t.Helper()
	uris := make([]string, len(paths))
	for i, p := range paths {
		uris[i] = transfer.FileURI(p)
	}
	err := obj.Call("org.freedesktop.FileManager1."+method, 0, uris, "token").Err
	hs.settle()
	return err
}

// A browser's "show in folder" (ShowItems) opens the folder with the
// downloaded file selected; ShowFolders opens a tab per folder, and
// ShowItemProperties shows the file's properties.
func TestFileManager1ShowsWhatItIsAsked(t *testing.T) {
	hs := newHarness(t)
	obj := hs.serve(t)
	if err := hs.call(t, obj, "ShowItems", hs.path("pics/a.png")); err != nil {
		t.Fatal(err)
	}
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("pics")}) || !slices.Equal(hs.selected(), []string{"a.png"}) {
		t.Errorf("ShowItems opened %v with %v selected, want pics with a.png", got, hs.selected())
	}
	if err := hs.call(t, obj, "ShowFolders", hs.path("docs"), hs.root); err != nil {
		t.Fatal(err)
	}
	if got := hs.tabs(); !slices.Equal(got, []string{hs.path("pics"), hs.path("docs"), hs.root}) {
		t.Errorf("ShowFolders left the tabs on %v", got)
	}
	if err := hs.call(t, obj, "ShowItemProperties", hs.path("b.txt")); err != nil {
		t.Fatal(err)
	}
	if q := hs.asking(); q == nil || q.Title != "b.txt" {
		t.Error("ShowItemProperties showed no properties")
	}
}

// A request naming no file, or a file that is not local, is refused
// whole and shows nothing.
func TestFileManager1RefusesWhatIsNoLocalFile(t *testing.T) {
	hs := newHarness(t)
	obj := hs.serve(t)
	for _, uris := range [][]string{nil, {"https://example.org/a.png"}, {transfer.FileURI(hs.path("b.txt")), "sftp://host/x"}} {
		err := obj.Call("org.freedesktop.FileManager1.ShowItems", 0, append([]string{}, uris...), "").Err
		var de dbus.Error
		if !errors.As(err, &de) || de.Name != dbusx.ErrInvalidArgs {
			t.Errorf("ShowItems %v answered %v, want InvalidArgs", uris, err)
		}
	}
	hs.settle()
	if len(hs.tabs()) != 0 {
		t.Errorf("refused requests opened %v", hs.tabs())
	}
}
