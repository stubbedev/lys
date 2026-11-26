// Package locale is lys-files' messages, in the desktop's language.
package locale

import (
	"embed"
	"io/fs"
	"sync"

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

var T = sync.OnceValue(func() *i18n.Loader {
	l, err := i18n.Select(Assets(), i18n.RequestedLanguages())
	if err != nil {
		panic(err)
	}
	return l
})
