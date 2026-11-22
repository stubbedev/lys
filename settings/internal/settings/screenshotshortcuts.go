package settings

import "github.com/stubbedev/lys/desktop/config"

const screenshotShortcutsKey = "settings-screenshot-shortcuts"

type screenshotCapture struct{ sub, key string }

var screenshotCaptures = []screenshotCapture{
	{"region", "settings-action-capture-region"},
	{"output", "settings-action-capture-output"},
	{"window", "settings-action-capture-window"},
	{"text", "settings-action-capture-text"},
	{"qr", "settings-action-capture-qr"},
}

func (c screenshotCapture) command() string { return "lys-shell screenshot " + c.sub }

func (c screenshotCapture) bind() string { return execAction + " " + c.command() }

func screenshotShortcutsRow() rowSpec {
	return wmField(bindsKey, withKey(screenshotShortcutsKey), func(r *rowSpec) {
		r.action = true
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			recorders := make([]bindRecorder, len(screenshotCaptures))
			for i, c := range screenshotCaptures {
				recorders[i] = bindRecorder{action: c.bind(), key: c.key}
			}
			return newBindRecorders(k, s, recorders)
		}
	})
}
