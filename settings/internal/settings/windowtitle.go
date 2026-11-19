package settings

import (
	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const windowTitleFormatPath = "modules.window-title.format"

var windowTitleShows = []struct{ format, key, icon string }{
	{"{{ app }}", "settings-window-title-show-app", "ld-app-window-symbolic"},
	{"{{ title }}", "settings-window-title-show-title", "ld-type-symbolic"},
	{"{{ app }}: {{ title }}", "settings-window-title-show-both", "ld-layers-symbolic"},
}

var windowTitleShow = func(r *rowSpec) {
	r.key = "settings-window-title-show"
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		values := make([]string, len(windowTitleShows))
		labels := make([]string, len(windowTitleShows))
		pictures := make(map[string]picture, len(windowTitleShows))
		for i, show := range windowTitleShows {
			values[i], labels[i] = show.format, locale.T().Get(show.key)
			pictures[show.format] = picture{icon: show.icon}
		}
		return newCards(k, s, values, labels, pictures)
	}
}
