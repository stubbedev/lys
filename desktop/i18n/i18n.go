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
// Each domain negotiates the desktop locale (LANGUAGE, LC_ALL,
// LC_MESSAGES, LANG) against its embedded locales once, on first use,
// with en-US as the always-loaded fallback. Lookups fall through the
// negotiated languages in order; a missing message yields i18n-embed's
// "No localization for id" marker rather than an empty string.
//
// A loader formats integers, byte sizes and numeric dates and times by
// the CLDR conventions of its most preferred negotiated catalogue
// (Integer, ByteSize, Time, DateTime), for a caller to pass as a
// string argument.
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

// domain lazily builds one loader from the desktop locale.
type domain struct {
	assets Assets
	once   sync.Once
	loader *Loader
}

func (d *domain) get() *Loader {
	d.once.Do(func() {
		l, err := Select(d.assets, requestedLanguages(os.Getenv))
		if err != nil {
			// The embedded fallback is validated by tests; failing here
			// is a build defect.
			panic(err)
		}
		d.loader = l
	})
	return d.loader
}

var (
	shellDomain    = &domain{assets: NewAssets(shelllocales.FS)}
	settingsDomain = &domain{assets: NewAssets(settingslocales.FS)}
	greeterDomain  = &domain{assets: NewFileAssets(greeterlocales.FS, "lys-greeter.ftl")}
)

// Shell returns the shell-domain loader.
func Shell() *Loader { return shellDomain.get() }

// Settings returns the settings-domain loader.
func Settings() *Loader { return settingsDomain.get() }

// ConfigAssets is the config's field labels and common messages, for
// a program that edits the config and adds messages of its own.
func ConfigAssets() Assets { return settingsDomain.assets }

// RequestedLanguages is the desktop locale's languages, as every
// domain negotiates them.
func RequestedLanguages() []LangID { return requestedLanguages(os.Getenv) }

// Greeter returns the greeter-domain loader.
func Greeter() *Loader { return greeterDomain.get() }

// T formats a shell message.
func T(id string, args ...Arg) string { return Shell().Get(id, args...) }

// Attr formats a shell message attribute.
func Attr(id, attr string, args ...Arg) string { return Shell().Attr(id, attr, args...) }
