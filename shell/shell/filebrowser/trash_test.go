package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// Trashed files list under their original names, typed by them; an
// orphan or malformed info leaves its file out, a trash that does not
// exist lists nothing, and folder modes list only folders.
func TestListTrash(t *testing.T) {
	home, mount := filepath.Join(t.TempDir(), "Trash"), filepath.Join(t.TempDir(), ".Trash-1000")
	report := browsertest.Trashed(t, home, "report.2", browsertest.TrashInfo("/home/u/My%20Report.pdf"))
	browsertest.Trashed(t, mount, "photo.png", browsertest.TrashInfo("pics/photo.png"))
	browsertest.Trashed(t, home, "orphan.txt", "")
	browsertest.Trashed(t, home, "badgroup", "[Desktop Entry]\nPath=/x\n")
	browsertest.Trashed(t, home, "nopath", "[Trash Info]\nDeletionDate=2026-10-01T12:00:00\n")
	browsertest.Trashed(t, home, "badescape", browsertest.TrashInfo("/home/u/%zz"))
	if err := os.MkdirAll(filepath.Join(home, "files", "folder"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "info", "folder.trashinfo"), []byte(browsertest.TrashInfo("/home/u/Old%20Folder")), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ListTrash(testMounts, []string{home, filepath.Join(t.TempDir(), "none"), mount}, FilesAndFolders)
	if err != nil {
		t.Fatal(err)
	}
	got := names(entries)
	slices.Sort(got)
	if want := []string{"My Report.pdf", "Old Folder", "photo.png"}; !slices.Equal(got, want) {
		t.Errorf("trash lists %q, want %q", got, want)
	}
	i := slices.IndexFunc(entries, func(e Entry) bool { return e.Path == report })
	if i < 0 || (kindNamer{db: browsertest.MIME(t)}).kind(entries[i]) != "PDF" {
		t.Errorf("the report is %+v, want the trashed file typed by its original name", entries)
	}

	folders, err := ListTrash(testMounts, []string{home}, FoldersOnly)
	if err != nil || !slices.Equal(names(folders), []string{"Old Folder"}) {
		t.Errorf("folder mode lists %q, %v", names(folders), err)
	}
	if os.Geteuid() != 0 {
		if err := os.Chmod(filepath.Join(home, "files"), 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(home, "files"), 0o700) })
		if _, err := ListTrash(testMounts, []string{home}, FilesAndFolders); err == nil || !strings.Contains(err.Error(), filepath.Join(home, "files")) {
			t.Errorf("an unreadable trash = %v, want an error naming it", err)
		}
	}
}

func TestTrashCrumb(t *testing.T) {
	if crumbs := Crumbs(TrashLocation, "/home/u"); len(crumbs) != 1 || crumbs[0] != (Crumb{"Trash", TrashLocation, true}) {
		t.Errorf("trash crumbs = %v", crumbs)
	}
}
