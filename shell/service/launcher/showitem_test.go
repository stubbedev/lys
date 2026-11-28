package launcher

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

type fileManager struct{ shown chan []string }

func (f *fileManager) ShowItems(uris []string, _ string) *dbus.Error {
	f.shown <- uris
	return nil
}

func TestShowItemAsksTheFileManagerToShowTheFile(t *testing.T) {
	dbustest.Session(t)
	conn := dbustest.SessionConn(t)
	fm := &fileManager{shown: make(chan []string, 1)}
	if err := conn.Export(fm, "/org/freedesktop/FileManager1", "org.freedesktop.FileManager1"); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName("org.freedesktop.FileManager1", dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own the name: %v %v", reply, err)
	}
	if err := ShowItem(context.Background(), "/home/u/a b.txt"); err != nil {
		t.Fatal(err)
	}
	if got := <-fm.shown; !slices.Equal(got, []string{"file:///home/u/a%20b.txt"}) {
		t.Errorf("the file manager was asked to show %v", got)
	}
}

func TestShowItemWithoutAFileManagerSaysSo(t *testing.T) {
	dbustest.Session(t)
	err := ShowItem(context.Background(), "/home/u/a.txt")
	if err == nil || !strings.Contains(err.Error(), "show /home/u/a.txt in the file manager") {
		t.Errorf("no file manager = %v, want an error naming the file", err)
	}
}
