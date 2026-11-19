package settings

import (
	"cmp"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/shell/service/sysinfo"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const storageMountPath = "modules.storage.mount-point"

// mountedDisks lists the mounted filesystems the storage module can
// measure (a seam for tests).
var mountedDisks = sysinfo.Disks

var mountPoints = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		c := &diskList{k: k, slot: s}
		c.liveList = newLiveList(k, "settings-storage-reading", "settings-storage-reload", func() { c.load() })
		c.load()
		return c
	}
}

type diskList struct {
	*liveList
	k      *kit
	slot   slot
	disks  []sysinfo.Disk
	loaded bool
}

func (c *diskList) load() {
	c.startReading()
	async(c.k, mountedDisks, c.show)
}

func (c *diskList) show(disks []sysinfo.Disk) {
	slices.SortFunc(disks, func(a, b sysinfo.Disk) int { return cmp.Compare(a.MountPoint, b.MountPoint) })
	c.disks, c.loaded = disks, true
	c.doneReading()
	c.refresh()
}

func (c *diskList) chosen() []string {
	switch v := c.slot.get().(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, p := range v {
			if s, ok := p.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (c *diskList) refresh() {
	if !c.loaded {
		return
	}
	t := locale.T()
	chosen := c.chosen()
	c.list.Clear()
	rows := widget.NewItemGroup()
	for _, path := range chosen {
		if !slices.ContainsFunc(c.disks, func(d sysinfo.Disk) bool { return d.MountPoint == path }) {
			rows.Append(c.diskRow(path, t.Get("settings-storage-not-mounted"), chosen))
		}
	}
	for _, d := range c.disks {
		used := t.Get("settings-storage-disk-usage",
			i18n.Str("used", sysinfo.AutoBytes(d.UsedBytes)), i18n.Str("size", sysinfo.AutoBytes(d.TotalBytes)), i18n.Str("filesystem", d.Filesystem))
		rows.Append(c.diskRow(d.MountPoint, used, chosen))
	}
	c.list.Append(rows, false)
}

func (c *diskList) diskRow(path, detail string, chosen []string) *widget.Item {
	on := slices.Contains(chosen, path)
	row := widget.NewItem(c.k.face, path, detail)
	row.AddClass(css.Small, "storage-disk")
	row.SetEllipsize(true)
	row.SetMedia(c.k.icon("ld-hard-drive-symbolic"), false)
	sw := widget.NewSwitch(on)
	sw.SetAccessibleName(path)
	sw.SetEnabled(!on || len(chosen) > 1)
	sw.OnChanged = func(on bool) { c.choose(path, on) }
	row.AddAction(sw)
	return row
}

func (c *diskList) choose(path string, on bool) {
	chosen := slices.DeleteFunc(c.chosen(), func(p string) bool { return p == path })
	if on {
		chosen = append(chosen, path)
	}
	if len(chosen) == 1 {
		_ = c.slot.set(chosen[0])
	} else {
		out := make([]any, len(chosen))
		for i, p := range chosen {
			out[i] = p
		}
		_ = c.slot.set(out)
	}
	c.refresh()
}
