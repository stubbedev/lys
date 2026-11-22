package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/opener"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	webEnginePath = "launcher.search.web-engine"
	webURLPath    = "launcher.search.web-url"
)

const webSearchSample = "lys desktop"

func openWithDefaultApp(uri string) error {
	return opener.Open(uri, desktopentry.LaunchContext{})
}

type webEngine struct{ name, url string }

var webEngines = []webEngine{
	{"DuckDuckGo", "https://duckduckgo.com/?q={query}"},
	{"Google", "https://www.google.com/search?q={query}"},
	{"Bing", "https://www.bing.com/search?q={query}"},
	{"Brave Search", "https://search.brave.com/search?q={query}"},
	{"Startpage", "https://www.startpage.com/do/search?query={query}"},
	{"Ecosia", "https://www.ecosia.org/search?q={query}"},
	{"Kagi", "https://kagi.com/search?q={query}"},
	{"Qwant", "https://www.qwant.com/?q={query}"},
}

type webEngineEditor struct {
	*widget.Box
	k       *kit
	name    slot
	url     slot
	pick    *widget.Dropdown
	custom  *widget.Box
	inputs  []*liveText
	tried   *widget.Label
	chosen  bool
	syncing bool
}

var webEngineRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.also = []string{webURLPath}
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		return newWebEngineEditor(k, s, k.store.slot(webURLPath))
	}
}

func newWebEngineEditor(k *kit, name, url slot) *webEngineEditor {
	t := locale.T()
	labels := make([]string, 0, len(webEngines)+1)
	for _, e := range webEngines {
		labels = append(labels, e.name)
	}
	labels = append(labels, t.Get("settings-web-engine-custom"))
	e := &webEngineEditor{Box: gapBox(widget.Column), k: k, name: name, url: url}
	e.AddClass("web-engine-editor")
	e.pick = widget.NewDropdown(k.face, k.textPx(), labels, 0)
	e.pick.SetAccessibleName(t.Get("settings-web-engine-pick"))
	e.pick.OnSelect = e.choose
	e.custom = gapBox(widget.Column)
	for _, f := range []struct {
		s           slot
		label, hint string
	}{{name, "settings-web-engine-name", "Searx"}, {url, "settings-web-engine-url", "https://example.org/search?q={query}"}} {
		in := newLiveText(k, f.s, f.hint)
		in.SetAccessibleName(t.Get(f.label))
		line := gapBox(widget.Row)
		line.AppendAligned(k.label(t.Get(f.label), "card-form-label", css.Caption), false, widget.AlignCenter)
		line.Append(in, true)
		e.custom.Append(line, false)
		e.inputs = append(e.inputs, in)
	}
	e.tried = k.note("", "format-preview")
	e.tried.SetVisible(false)
	e.custom.AppendAligned(k.secondaryButton(t.Get("settings-web-engine-test"), e.test), false, widget.AlignStart)
	e.custom.Append(e.tried, false)
	e.AppendAligned(e.pick, false, widget.AlignStart)
	e.Append(e.custom, false)
	e.refresh()
	return e
}

func (e *webEngineEditor) known() int {
	cur := webEngine{e.name.text(), e.url.text()}
	return slices.Index(webEngines, cur)
}

func (e *webEngineEditor) choose(i int) {
	if e.syncing {
		return
	}
	if i >= len(webEngines) {
		e.chosen = true
		e.refresh()
		return
	}
	e.chosen = false
	if err := e.url.set(webEngines[i].url); err != nil {
		return
	}
	_ = e.name.set(webEngines[i].name)
	e.refresh()
}

func (e *webEngineEditor) refresh() {
	i := e.known()
	custom := i < 0 || e.chosen
	if custom {
		i = len(webEngines)
	}
	e.syncing = true
	e.pick.SetSelected(i)
	e.syncing = false
	e.custom.SetVisible(custom)
	for _, in := range e.inputs {
		in.refresh()
	}
}

func (e *webEngineEditor) test() {
	t := locale.T()
	e.tried.SetVisible(true)
	target, ok := config.FillQueryURL(e.url.text(), webSearchSample)
	if !ok {
		e.tried.SetText(t.Get("settings-web-engine-no-query", i18n.Str("token", config.QueryToken)))
		return
	}
	async(e.k, func() error { return e.k.openURI(target) }, func(err error) {
		if err != nil {
			e.tried.SetText(t.Get("settings-web-engine-test-failed", i18n.Str("reason", err.Error())))
			return
		}
		e.tried.SetText(t.Get("settings-web-engine-tested", i18n.Str("query", webSearchSample), i18n.Str("url", target)))
	})
}
