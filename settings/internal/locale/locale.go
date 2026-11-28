// Package locale is the app's messages: lys-shell's labels for its config
// fields with this app's own on top, in the desktop's language.
package locale

import (
	"embed"
	"io/fs"

	"github.com/stubbedev/lys/desktop/i18n"
)

//go:embed all:locales
var locales embed.FS

// Assets is every locale's messages.
func Assets() i18n.Assets {
	own, err := fs.Sub(locales, "locales")
	if err != nil {
		panic(err)
	}
	return i18n.ConfigAssets().With(own)
}

// T is the loader for the desktop's language.
var T = i18n.Lazy(Assets())
