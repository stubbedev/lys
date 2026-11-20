package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/portal/permissions"
)

type privacyResource struct {
	table  permissions.Table
	device string
	key    string
	icon   string
	core   bool
}

var (
	cameraResource     = privacyResource{table: permissions.Devices, device: "camera", key: "settings-privacy-resource-camera", icon: "ld-webcam-symbolic", core: true}
	microphoneResource = privacyResource{table: permissions.Devices, device: "microphone", key: "settings-privacy-resource-microphone", icon: "ld-mic-symbolic", core: true}
	screenResource     = privacyResource{table: permissions.ScreenCast, key: "settings-privacy-table-screencast", icon: "ld-monitor-symbolic", core: true}
	privacyResources   = []privacyResource{
		cameraResource,
		microphoneResource,
		{table: permissions.Devices, device: "speakers", key: "settings-privacy-resource-speakers", icon: "ld-speaker-symbolic"},
		{table: permissions.Devices, key: "settings-privacy-table-devices", icon: "ld-headset-symbolic"},
		screenResource,
		{table: permissions.Location, key: "settings-privacy-table-location", icon: "ld-globe-symbolic", core: true},
		{table: permissions.Screenshot, key: "settings-privacy-table-screenshot", icon: "ld-camera-symbolic"},
		{table: permissions.RemoteDesktop, key: "settings-privacy-table-remote-desktop", icon: "ld-laptop-symbolic"},
		{table: permissions.InputCapture, key: "settings-privacy-table-input-capture", icon: "ld-mouse-symbolic"},
		{table: permissions.Shortcuts, key: "settings-privacy-table-global-shortcuts", icon: "ld-keyboard-symbolic"},
		{table: permissions.Background, key: "settings-privacy-table-background", icon: "ld-layers-symbolic"},
		{table: permissions.Notifications, key: "settings-privacy-table-notifications", icon: "ld-bell-symbolic"},
		{table: permissions.USB, key: "settings-privacy-table-usb", icon: "ld-usb-symbolic"},
		{table: permissions.Wallpaper, key: "settings-privacy-table-wallpaper", icon: "ld-image-symbolic"},
		{table: permissions.Inhibit, key: "settings-privacy-table-inhibit", icon: "ld-coffee-symbolic"},
		{table: permissions.GameMode, key: "settings-privacy-table-gamemode", icon: "ld-gamepad-2-symbolic"},
		{table: permissions.Realtime, key: "settings-privacy-table-realtime", icon: "ld-cpu-symbolic"},
	}
)

func resourceOf(g permissions.Grant) (privacyResource, bool) {
	i := slices.IndexFunc(privacyResources, func(r privacyResource) bool {
		return r.table == g.Table && (r.device == "" || r.device == g.ID)
	})
	if i < 0 {
		return privacyResource{}, false
	}
	return privacyResources[i], true
}

type heldGrant struct {
	app   string
	grant permissions.Grant
}

func resourceGrants(apps []permissions.App, r privacyResource) []heldGrant {
	var out []heldGrant
	for _, app := range apps {
		for _, g := range app.Grants {
			if of, ok := resourceOf(g); ok && of == r {
				out = append(out, heldGrant{app: app.ID, grant: g})
			}
		}
	}
	return out
}

func countApps(held []heldGrant) int64 {
	var apps []string
	for _, h := range held {
		if !slices.Contains(apps, h.app) {
			apps = append(apps, h.app)
		}
	}
	return int64(len(apps))
}
