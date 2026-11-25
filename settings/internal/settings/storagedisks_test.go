package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/shell/service/sysinfo"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func diskListWith(t *testing.T, yaml string, disks ...sysinfo.Disk) (*kit, *checklist) {
	t.Helper()
	k := testKit(t, yaml)
	mountedDisks = func() []sysinfo.Disk { return slices.Clone(disks) }
	c, ok := fieldControl(k, k.store, pageRow(t, "storage", storageMountPath)).(*checklist)
	if !ok {
		t.Fatal("the storage page does not list the disks")
	}
	return k, c
}

var rootAndData = []sysinfo.Disk{
	{MountPoint: "/mnt/data", Filesystem: "ext4", TotalBytes: 1 << 40, UsedBytes: 1 << 39},
	{MountPoint: "/", Filesystem: "btrfs", TotalBytes: 1 << 39, UsedBytes: 1 << 37},
}

func TestTheStoragePageListsTheMountedDisksToChooseFrom(t *testing.T) {
	k, c := diskListWith(t, "", rootAndData...)
	root, data := switchNamed(t, c, "/"), switchNamed(t, c, "/mnt/data")
	if !root.On() || data.On() {
		t.Fatalf("by default / is on %v and /mnt/data %v, want / alone", root.On(), data.On())
	}
	if root.Enabled() {
		t.Error("the only disk chosen can be switched off")
	}
	data.Toggle()
	if got := k.store.value(storageMountPath); !reflect.DeepEqual(got, []any{"/", "/mnt/data"}) {
		t.Fatalf("adding /mnt/data stored %#v", got)
	}
	switchNamed(t, c, "/").Toggle()
	if got := k.store.value(storageMountPath); !reflect.DeepEqual(got, []any{"/mnt/data"}) {
		t.Errorf("dropping / stored %#v, want the one path left", got)
	}
}

func TestAChosenDiskThatIsNotMountedStaysListed(t *testing.T) {
	_, c := diskListWith(t, "modules:\n  storage:\n    mount-point: ['/', '/media/usb']\n", rootAndData...)
	if sw := switchNamed(t, c, "/media/usb"); !sw.On() || !sw.Enabled() {
		t.Errorf("the unmounted chosen disk: on %v, enabled %v", sw.On(), sw.Enabled())
	}
	if !slices.Contains(treetest.Labels(c), locale.T().Get("settings-storage-not-mounted")) {
		t.Error("the unmounted disk does not say so")
	}
}

// A disk's usage read its sizes through an English-only formatter. It
// reads them in the language's own byte sizes.
func TestADisksUsageReadsTheLanguagesByteSizes(t *testing.T) {
	_, c := diskListWith(t, "", rootAndData...)
	tr := locale.T()
	want := tr.Get("settings-storage-disk-usage", i18n.Str("used", tr.ByteSize(1<<37)), i18n.Str("size", tr.ByteSize(1<<39)), i18n.Str("filesystem", "btrfs"))
	if !slices.Contains(treetest.Labels(c), want) {
		t.Errorf("no disk reads %q among %q", want, treetest.Labels(c))
	}
}
