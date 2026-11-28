package files

import (
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/shell/shell/filebrowser"

	"github.com/stubbedev/lys/files/internal/locale"
)

// statusText is the status line of the tab showing.
func (hs *harness) statusText() string {
	var text string
	hs.run(func() { text = hs.f.active().status.Text() })
	return text
}

// waitStatus waits until the status line of the tab showing reads
// want, which a change on disk reaches through the folder's watch.
func (hs *harness) waitStatus(t *testing.T, want string) {
	t.Helper()
	waittest.UntilShowing(t, func() bool {
		hs.flush()
		return hs.statusText() == want
	}, "the status line to read "+want, func() string { return "it reads " + hs.statusText() })
}

func itemsText(n int) string {
	return locale.T().Get("files-status-items", i18n.Int("count", int64(n)))
}

func selectedText(n, total int) string {
	return locale.T().Get("files-status-selected", i18n.Int("count", int64(n)), i18n.Int("total", int64(total)))
}

func sizedText(n, total int, size int64) string {
	return locale.T().Get("files-status-size", i18n.Str("selection", selectedText(n, total)), i18n.Str("size", filebrowser.HumanSize(size)))
}

// The status line counts the items of the folder, or the selection of
// them and the size of its files, a folder an item without a size, and
// follows the selection and the files made and removed on disk; in the
// trash too.
func TestTheStatusLineCountsTheItemsAndTheSelection(t *testing.T) {
	hs := newHarness(t)
	hs.write(t, "b.txt", "hello")
	hs.write(t, "a.png", "png")
	hs.open(hs.root)
	hs.waitStatus(t, itemsText(4))
	hs.pick(t, "b.txt")
	hs.waitStatus(t, sizedText(1, 4, 5))
	hs.pick(t, "a.png", "b.txt", "docs")
	hs.waitStatus(t, sizedText(3, 4, 8))
	hs.pick(t, "docs", "pics")
	hs.waitStatus(t, selectedText(2, 4))
	hs.pick(t)
	hs.waitStatus(t, itemsText(4))
	hs.write(t, "c.txt", "c")
	hs.waitStatus(t, itemsText(5))
	if err := os.Remove(hs.path("c.txt")); err != nil {
		t.Fatal(err)
	}
	hs.waitStatus(t, itemsText(4))
	hs.pick(t, "b.txt")
	hs.key(xkb.KeyDelete, 0, nil)
	hs.do(func() { hs.f.active().b.GoTo(filebrowser.TrashLocation) })
	hs.waitStatus(t, itemsText(1))
}

// A folder being read says nothing yet; an empty one says so.
func TestStatus(t *testing.T) {
	if got := status(nil, 0, true); got != "" {
		t.Errorf("a folder being read says %q", got)
	}
	if got := status(nil, 0, false); got != itemsText(0) || strings.Contains(got, "0") {
		t.Errorf("an empty folder says %q", got)
	}
	dir := []filebrowser.Entry{{Path: "/d", IsDir: true, Size: 4096}}
	if got := status(dir, 3, false); got != selectedText(1, 3) {
		t.Errorf("a folder selected says %q", got)
	}
}
