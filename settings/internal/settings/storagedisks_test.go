package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/shell/service/sysinfo"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func diskListWith(t *testing.T, yaml string, disks ...sysinfo.Disk) (*kit, *diskList) {
	t.Helper()
	was := mountedDisks
	mountedDisks = func() []sysinfo.Disk { return slices.Clone(disks) }
	t.Cleanup(func() { mountedDisks = was })
	k := testKit(t, yaml)
	c, ok := fieldControl(k, k.store, pageRow(t, "storage", storageMountPath)).(*diskList)
	if !ok {
		t.Fatal("the storage page does not list the disks")
	}
	return k, c
}

var rootAndData = []sysinfo.Disk{
	{MountPoint: "/mnt/data", Filesystem: "ext4", TotalBytes: 1 << 40, UsedBytes: 1 << 39},
	{MountPoint: "/", Filesystem: "btrfs", TotalBytes: 1 << 39, UsedBytes: 1 << 37},
}

func diskSwitch(t *testing.T, c *diskList, path string) *widget.Switch {
	t.Helper()
	for _, sw := range treetest.All[*widget.Switch](c) {
		if sw.AccessibleName() == path {
			return sw
		}
	}
	t.Fatalf("no switch for %s", path)
	return nil
}

func TestTheStoragePageListsTheMountedDisksToChooseFrom(t *testing.T) {
	k, c := diskListWith(t, "", rootAndData...)
	root, data := diskSwitch(t, c, "/"), diskSwitch(t, c, "/mnt/data")
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
	diskSwitch(t, c, "/").Toggle()
	if got := k.store.value(storageMountPath); got != "/mnt/data" {
		t.Errorf("dropping / stored %#v, want the one path left", got)
	}
}

func TestAChosenDiskThatIsNotMountedStaysListed(t *testing.T) {
	_, c := diskListWith(t, "modules:\n  storage:\n    mount-point: ['/', '/media/usb']\n", rootAndData...)
	if sw := diskSwitch(t, c, "/media/usb"); !sw.On() || !sw.Enabled() {
		t.Errorf("the unmounted chosen disk: on %v, enabled %v", sw.On(), sw.Enabled())
	}
	if !slices.Contains(treetest.Labels(c), locale.T().Get("settings-storage-not-mounted")) {
		t.Error("the unmounted disk does not say so")
	}
}
