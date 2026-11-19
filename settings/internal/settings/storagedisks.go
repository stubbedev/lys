package settings

import (
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/shell/service/sysinfo"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const storageMountPath = "modules.storage.mount-point"

// mountedDisks lists the mounted filesystems the storage module can
// measure (a seam for tests).
var mountedDisks = sysinfo.Disks

var mountPoints = checklistRow(checklistSpec{
	readingKey: "settings-storage-reading", reloadKey: "settings-storage-reload",
	missingKey: "settings-storage-not-mounted", emptyKey: "settings-storage-none",
	icon: "ld-hard-drive-symbolic", keepOne: true,
	read: func() checkListing {
		t := locale.T()
		disks := mountedDisks()
		items := make([]checkItem, len(disks))
		for i, d := range disks {
			items[i] = checkItem{value: d.MountPoint, detail: t.Get("settings-storage-disk-usage",
				i18n.Str("used", sysinfo.AutoBytes(d.UsedBytes)), i18n.Str("size", sysinfo.AutoBytes(d.TotalBytes)), i18n.Str("filesystem", d.Filesystem))}
		}
		return checkListing{items: items}
	},
})
