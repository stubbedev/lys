// Package locale is lys-viewer's messages, in the desktop's language.
package locale

import (
	"embed"
	"io/fs"

	"github.com/stubbedev/lys/desktop/i18n"
)

//go:embed all:locales
var locales embed.FS

func Assets() i18n.Assets {
	own, err := fs.Sub(locales, "locales")
	if err != nil {
		panic(err)
	}
	return i18n.NewAssets(own)
}

// T is the loader for the desktop's language.
var T = i18n.Lazy(Assets())
