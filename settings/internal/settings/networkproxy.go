package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type proxyState struct {
	profile network.Profile
	proxy   network.Proxy
}

var proxyMethodLabels = map[network.ProxyMethod]string{
	network.ProxyNone: "settings-proxy-none",
	network.ProxyAuto: "settings-proxy-auto",
}

var pacProblems = map[error]string{
	network.ErrPACURLInvalid: "settings-proxy-pac-invalid",
	network.ErrPACURLHost:    "settings-proxy-pac-host",
	network.ErrPACURLPath:    "settings-proxy-pac-path",
}

func pacProblem(err error) (string, bool) {
	for sentinel, key := range pacProblems {
		if errors.Is(err, sentinel) {
			return locale.T().Get(key), true
		}
	}
	return "", false
}

type proxyForm struct {
	*widget.Box
	p       *networkPanel
	none    *widget.Label
	row     *widget.Box
	title   *widget.Label
	methods *widget.ToggleGroup
	auto    *widget.Box
	pac     *widget.Entry
	field   *widget.Field
	path    dbus.ObjectPath
	shown   string
	syncing bool
}

func newProxyForm(p *networkPanel) *proxyForm {
	k := p.k
	t := locale.T()
	f := &proxyForm{Box: gapBox(widget.Column), p: p}
	f.AddClass("proxy-form")
	f.none = wrapped(k.label(t.Get("settings-proxy-no-connection"), css.Caption, css.DimLabel))
	f.Append(f.none, false)
	f.methods = widget.NewToggleGroup()
	f.methods.SetAccessibleName(t.Get("settings-proxy-section"))
	for _, m := range network.ProxyMethods {
		f.methods.Append(k.label(t.Get(proxyMethodLabels[m])))
	}
	f.methods.OnChanged = func(i int) {
		if f.syncing || i < 0 || i >= len(network.ProxyMethods) {
			return
		}
		f.choose(network.ProxyMethods[i])
	}
	f.title = k.label("", "setting-label")
	f.row = gapBox(widget.Row)
	f.row.SetElement("item")
	f.row.AddClass(css.Small, "setting-row")
	f.row.AppendAligned(f.title, true, widget.AlignCenter)
	f.row.AppendAligned(f.methods, false, widget.AlignCenter)
	group := widget.NewItemGroup()
	group.Append(f.row)
	f.Append(group, false)
	f.pac = k.entry()
	f.pac.SetPlaceholder(t.Get("settings-proxy-pac-hint"))
	f.pac.SetAccessibleName(t.Get("settings-proxy-pac"))
	f.pac.OnChanged = func(string) { f.field.SetError("") }
	f.pac.OnActivate = func(string) { f.choose(network.ProxyAuto) }
	f.field = widget.NewField(k.face, t.Get("settings-proxy-pac"), f.pac)
	f.auto = gapBox(widget.Column)
	f.auto.Append(f.field, false)
	f.auto.AppendAligned(k.button(k.label(t.Get("settings-proxy-apply")), func() { f.choose(network.ProxyAuto) }, css.Small, css.SuggestedAction), false, widget.AlignStart)
	f.Append(f.auto, false)
	f.show(nil)
	return f
}

func (f *proxyForm) show(st *proxyState) {
	f.none.SetVisible(st == nil)
	f.row.SetVisible(st != nil)
	f.auto.SetVisible(false)
	if st == nil {
		f.path = ""
		return
	}
	f.title.SetText(locale.T().Get("settings-proxy-for", i18n.Str("name", st.profile.ID)))
	method := st.proxy.Method()
	f.syncing = true
	f.methods.SetActive(slices.Index(network.ProxyMethods, method))
	f.syncing = false
	f.auto.SetVisible(method == network.ProxyAuto)
	if f.path != st.profile.Path || f.shown != st.proxy.PACURL() {
		f.path, f.shown = st.profile.Path, st.proxy.PACURL()
		f.pac.SetText(f.shown)
		f.field.SetError("")
	}
}

func (f *proxyForm) choose(method network.ProxyMethod) {
	if f.path == "" {
		return
	}
	proxy, err := f.proxy(method)
	if err != nil {
		f.auto.SetVisible(true)
		msg, ok := pacProblem(err)
		if !ok {
			msg = err.Error()
		}
		f.field.SetError(msg)
		return
	}
	f.field.SetError("")
	path := f.path
	f.p.run(func(ctx context.Context) error { return f.p.ctl.setProxy(ctx, path, proxy) })
}

func (f *proxyForm) proxy(method network.ProxyMethod) (network.Proxy, error) {
	switch method {
	case network.ProxyNone:
		return network.NoProxy(), nil
	case network.ProxyAuto:
		return network.AutoProxy(f.pac.Text())
	}
	return network.Proxy{}, fmt.Errorf("settings: proxy method %d", method)
}
