// Package i18n is lys's Fluent localization: a loader, bundle and
// parser following i18n-embed 0.16 (FluentLanguageLoader),
// fluent-bundle 0.16 and fluent-syntax 0.12, language negotiation by
// language subtag, and CLDR cardinal plural rules for every catalogue
// it ships: en-US, de, es, fr, it, nl and pt (European Portuguese).
//
// Three domains exist:
//
//   - the shell domain: bar labels, dropdowns, OSD, notification
//     popups. T and Attr read it.
//   - the settings domain: the config's field labels, which
//     lys-settings adds its own messages to (ConfigAssets). Settings
//     returns it.
//   - the greeter domain: the login screen's labels. Greeter returns
//     it.
//
// Each domain negotiates the desktop locale (LANGUAGE, then the first
// of LC_ALL, LC_MESSAGES and LANG) against its embedded locales once,
// on first use, with en-US as the always-loaded fallback. Lookups fall through the
// negotiated languages in order; a missing message yields i18n-embed's
// "No localization for id" marker rather than an empty string.
//
// A loader formats integers, binary and decimal byte sizes and numeric
// dates and times by the CLDR conventions of its most preferred
// negotiated catalogue (Integer, ByteSize, DecimalByteSize, Time,
// DateTime), for a caller to pass as a string argument.
//
// Interpolated placeables are wrapped in Unicode isolation marks
// (U+2068 ... U+2069) exactly as fluent-bundle does by default, which
// i18n-embed never turns off.
package i18n

import (
	"os"
	"sync"

	greeterlocales "github.com/stubbedev/lys/desktop/i18n/locales/greeter"
	settingslocales "github.com/stubbedev/lys/desktop/i18n/locales/settings"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

// Shipped is every language each catalogue carries, sorted by name as
// Assets.Available lists them.
func Shipped() []LangID {
	return []LangID{MustLangID("de"), Fallback, MustLangID("es"), MustLangID("fr"), MustLangID("it"), MustLangID("nl"), MustLangID("pt")}
}

// Lazy is the loader of assets for the desktop locale, negotiated on
// its first call. The embedded fallback is validated by tests, so a
// failure to load is a build defect and panics.
func Lazy(assets Assets) func() *Loader {
	return sync.OnceValue(func() *Loader {
		l, err := Select(assets, RequestedLanguages())
		if err != nil {
			panic(err)
		}
		return l
	})
}

var (
	settingsAssets = NewAssets(settingslocales.FS)
	shellDomain    = Lazy(NewAssets(shelllocales.FS))
	settingsDomain = Lazy(settingsAssets)
	greeterDomain  = Lazy(NewFileAssets(greeterlocales.FS, "lys-greeter.ftl"))
)

// Shell returns the shell-domain loader.
func Shell() *Loader { return shellDomain() }

// Settings returns the settings-domain loader.
func Settings() *Loader { return settingsDomain() }

// ConfigAssets is the config's field labels and common messages, for
// a program that edits the config and adds messages of its own.
func ConfigAssets() Assets { return settingsAssets }

// RequestedLanguages is the desktop locale's languages, as every
// domain negotiates them.
func RequestedLanguages() []LangID { return requestedLanguages(os.Getenv) }

// Greeter returns the greeter-domain loader.
func Greeter() *Loader { return greeterDomain() }

// T formats a shell message.
func T(id string, args ...Arg) string { return Shell().Get(id, args...) }

// Attr formats a shell message attribute.
func Attr(id, attr string, args ...Arg) string { return Shell().Attr(id, attr, args...) }
