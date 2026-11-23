package settings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/proxyenv"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const wmEnvPath = "env"

type proxyState struct {
	profile network.Profile
	proxy   network.Proxy
}

type proxyChoice int

const (
	proxyChoiceNone proxyChoice = iota
	proxyChoiceAuto
	proxyChoiceManual
)

var proxyChoiceLabels = map[proxyChoice]string{
	proxyChoiceNone:   "settings-proxy-none",
	proxyChoiceAuto:   "settings-proxy-auto",
	proxyChoiceManual: "settings-proxy-manual",
}

var proxyServerLabels = map[proxyenv.Kind]string{
	proxyenv.HTTP:  "settings-proxy-http",
	proxyenv.HTTPS: "settings-proxy-https",
	proxyenv.SOCKS: "settings-proxy-socks",
}

var proxyDefaultPorts = map[proxyenv.Kind]uint16{
	proxyenv.HTTP:  3128,
	proxyenv.HTTPS: 3128,
	proxyenv.SOCKS: 1080,
}

var proxyExclusionExamples = []string{"localhost", "127.0.0.1", "::1"}

var proxyProblems = map[error]string{
	network.ErrPACURLInvalid: "settings-proxy-pac-invalid",
	network.ErrPACURLHost:    "settings-proxy-pac-host",
	network.ErrPACURLPath:    "settings-proxy-pac-path",
	proxyenv.ErrHost:         "settings-proxy-host-invalid",
	proxyenv.ErrPort:         "settings-proxy-port-invalid",
	proxyenv.ErrNoServer:     "settings-proxy-no-server",
	proxyenv.ErrExclusion:    "settings-proxy-exclude-invalid",
	proxyenv.ErrMismatch:     "settings-proxy-env-mismatch",
	proxyenv.ErrScheme:       "settings-proxy-env-scheme",
	proxyenv.ErrURL:          "settings-proxy-env-url",
}

// proxyProblem says what is wrong with a proxy that failed with err,
// in the page's words.
func proxyProblem(err error) string {
	if e, ok := errors.AsType[*proxyInConfigError](err); ok {
		return locale.T().Get("settings-proxy-env-config", i18n.Str("names", strings.Join(e.names, ", ")))
	}
	for sentinel, key := range proxyProblems {
		if errors.Is(err, sentinel) {
			return locale.T().Get(key)
		}
	}
	return locale.T().Get("settings-proxy-not-applied")
}

type proxyServerInput struct {
	host  *widget.Entry
	port  *widget.SpinButton
	field *widget.Field
}

type proxyForm struct {
	*widget.Box
	p        *networkPanel
	none     *widget.Label
	row      *settingRow
	choices  []proxyChoice
	methods  *choicePick
	auto     *widget.Box
	pac      *widget.Entry
	field    *widget.Field
	wm       *wmBackend
	noManual *widget.Label
	manual   *widget.Box
	servers  [len(proxyenv.Kinds)]proxyServerInput
	exclude  []any
	chips    *nameChips
	problem  *inlineState
	status   *wmStatus
	st       *proxyState
	path     dbus.ObjectPath
	shown    string
	applied  proxyenv.Manual
	picked   bool
}

func newProxyForm(p *networkPanel) *proxyForm {
	k := p.k
	t := locale.T()
	f := &proxyForm{Box: gapBox(widget.Column), p: p, choices: []proxyChoice{proxyChoiceNone, proxyChoiceAuto}}
	f.AddClass("proxy-form")
	f.none = k.note(t.Get("settings-proxy-no-connection"))
	f.Append(f.none, false)
	f.noManual = k.note("")
	switch w, err := proxyWM(k); {
	case err != nil:
		f.noManual.SetText(err.Error())
	default:
		f.wm = w
		f.choices = append(f.choices, proxyChoiceManual)
	}
	labels := make([]string, len(f.choices))
	for i, c := range f.choices {
		labels[i] = t.Get(proxyChoiceLabels[c])
	}
	f.methods = newChoicePick(k, labels, 0)
	f.methods.SetAccessibleName(t.Get("settings-proxy-section"))
	f.methods.OnSelect = func(i int) { f.choose(f.choices[i]) }
	f.row = k.row("", "").withControl(f.methods)
	group := widget.NewItemGroup()
	group.Append(f.row)
	f.Append(group, false)
	f.Append(f.noManual, false)
	f.pac = k.entry()
	f.pac.SetPlaceholder(t.Get("settings-proxy-pac-hint"))
	f.pac.SetAccessibleName(t.Get("settings-proxy-pac"))
	f.pac.OnChanged = func(string) { f.field.SetError("") }
	f.pac.OnActivate = func(string) { f.choose(proxyChoiceAuto) }
	f.field = widget.NewField(k.face, t.Get("settings-proxy-pac"), f.pac)
	f.auto = gapBox(widget.Column)
	f.auto.Append(f.field, false)
	f.auto.AppendAligned(k.primaryButton(t.Get("settings-proxy-apply"), func() { f.choose(proxyChoiceAuto) }), false, widget.AlignStart)
	f.Append(f.auto, false)
	if f.wm != nil {
		f.manual = f.manualForm()
		f.Append(f.manual, false)
	}
	f.show(nil)
	return f
}

func proxyWM(k *kit) (*wmBackend, error) {
	t := locale.T()
	if !k.parts.wm {
		return nil, errors.New(t.Get("settings-proxy-manual-missing"))
	}
	w, err := k.wmBackend()
	if err != nil {
		return nil, errors.New(t.Get("settings-proxy-manual-unread", i18n.Str("reason", err.Error())))
	}
	return w, nil
}

func (f *proxyForm) manualForm() *widget.Box {
	k := f.p.k
	t := locale.T()
	box := gapBox(widget.Column)
	box.AddClass("proxy-manual")
	box.Append(k.note(t.Get("settings-proxy-manual-note")), false)
	for _, kind := range proxyenv.Kinds {
		in := &f.servers[kind]
		in.host = k.entry()
		in.host.SetPlaceholder(t.Get("settings-proxy-host-hint"))
		in.host.SetAccessibleName(t.Get(proxyServerLabels[kind]))
		in.host.OnChanged = func(string) { in.field.SetError(""); f.problem.Clear() }
		in.host.OnActivate = func(string) { f.apply() }
		in.port = widget.NewSpinButton(k.face, k.textPx(), widget.Current().Text, 1, 65535, 1, 0)
		in.port.SetAccessibleName(t.Get("settings-proxy-port"))
		in.port.SetValue(float64(proxyDefaultPorts[kind]))
		line := gapBox(widget.Row)
		line.AppendAligned(in.host, true, widget.AlignCenter)
		line.AppendAligned(in.port, false, widget.AlignCenter)
		in.field = widget.NewField(k.face, t.Get(proxyServerLabels[kind]), line)
		box.Append(in.field, false)
	}
	f.chips = newNameChips(k, slot{
		get:   func() any { return f.exclude },
		set:   func(v any) error { f.exclude, _ = v.([]any); return nil },
		unset: func() { f.exclude = nil },
	}, nameChipsSpec{
		offered: func(*kit) func() offeredNames {
			return func() offeredNames { return offeredNames{names: proxyExclusionExamples} }
		},
		placeholder: "settings-proxy-exclude-hint",
		addKey:      "settings-proxy-exclude-add",
		problem:     proxyExclusionProblem,
	})
	box.Append(widget.NewField(k.face, t.Get("settings-proxy-exclude"), f.chips), false)
	f.problem = k.inlineState()
	box.Append(f.problem, false)
	box.AppendAligned(k.primaryButton(t.Get("settings-proxy-apply"), f.apply), false, widget.AlignStart)
	f.status = newWMStatus(k, f.wm)
	box.Append(f.status, false)
	return box
}

func proxyExclusionProblem(name string, listed []string) string {
	if slices.Contains(listed, name) {
		return "settings-proxy-exclude-listed"
	}
	if _, err := proxyenv.ParseExclusion(name); err != nil {
		return "settings-proxy-exclude-invalid"
	}
	return ""
}

func (f *proxyForm) configChanged() {
	if f.status != nil {
		f.status.refresh()
	}
	f.show(f.st)
}

func (f *proxyForm) show(st *proxyState) {
	f.st = st
	f.none.SetVisible(st == nil)
	f.row.SetVisible(st != nil)
	f.noManual.SetVisible(st != nil && f.wm == nil)
	f.auto.SetVisible(false)
	if f.manual != nil {
		f.manual.SetVisible(false)
	}
	if st == nil {
		f.path = ""
		return
	}
	f.row.SetTitle(locale.T().Get("settings-proxy-for", i18n.Str("name", st.profile.ID)))
	choice := proxyChoiceOf(st.proxy.Method())
	if f.wm != nil {
		set := f.showManual()
		if set || f.picked {
			choice = proxyChoiceManual
		}
	}
	f.methods.show(slices.Index(f.choices, choice))
	f.auto.SetVisible(choice == proxyChoiceAuto)
	if f.manual != nil {
		f.manual.SetVisible(choice == proxyChoiceManual)
	}
	if f.path != st.profile.Path || f.shown != st.proxy.PACURL() {
		f.path, f.shown = st.profile.Path, st.proxy.PACURL()
		f.pac.SetText(f.shown)
		f.field.SetError("")
	}
}

func proxyChoiceOf(m network.ProxyMethod) proxyChoice {
	if m == network.ProxyAuto {
		return proxyChoiceAuto
	}
	return proxyChoiceNone
}

func (f *proxyForm) showManual() bool {
	env, err := wmEnv(f.wm)
	if err != nil {
		f.problem.Error(locale.T().Get("settings-proxy-env-unreadable"), err, nil)
		return false
	}
	m, set, err := proxyenv.FromEnv(env)
	switch {
	case !set:
		return false
	case err != nil:
		f.problem.Error(locale.T().Get("settings-proxy-env-unread", i18n.Str("problem", proxyProblem(err))), err, nil)
	case !m.Equal(f.applied):
		f.fill(m)
	}
	return true
}

func (f *proxyForm) fill(m proxyenv.Manual) {
	f.applied = m
	for _, kind := range proxyenv.Kinds {
		in := f.servers[kind]
		s, ok := m.Server(kind)
		port := proxyDefaultPorts[kind]
		host := ""
		if ok {
			host, port = s.Host(), s.Port()
		}
		in.host.SetText(host)
		in.port.SetValue(float64(port))
		in.field.SetError("")
	}
	exclusions := m.Exclusions()
	f.exclude = make([]any, len(exclusions))
	for i, x := range exclusions {
		f.exclude[i] = x.String()
	}
	f.chips.refresh()
	f.problem.Clear()
}

func wmEnv(w *wmBackend) (map[string]string, error) {
	raw, _ := w.value(wmEnvPath).(map[string]any)
	env := make(map[string]string, len(raw))
	for name, v := range raw {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("settings: lys-wm's env %s is a %T, not a string", name, v)
		}
		env[name] = s
	}
	return env, nil
}

func (f *proxyForm) choose(choice proxyChoice) {
	if f.path == "" {
		return
	}
	if choice == proxyChoiceManual {
		f.picked = true
		f.show(f.st)
		return
	}
	proxy, err := f.proxy(choice)
	if err != nil {
		f.auto.SetVisible(true)
		f.field.SetError(proxyProblem(err))
		return
	}
	f.field.SetError("")
	f.picked = false
	if f.wm != nil && !f.writeEnv(nil) {
		return
	}
	f.setProxy(proxy)
}

func (f *proxyForm) setProxy(proxy network.Proxy) {
	path := f.path
	f.p.run(func(ctx context.Context) error { return f.p.ctl.setProxy(ctx, path, proxy) })
}

func (f *proxyForm) proxy(choice proxyChoice) (network.Proxy, error) {
	switch choice {
	case proxyChoiceNone:
		return network.NoProxy(), nil
	case proxyChoiceAuto:
		return network.AutoProxy(f.pac.Text())
	case proxyChoiceManual:
	}
	return network.Proxy{}, fmt.Errorf("settings: proxy choice %d has no NetworkManager proxy", choice)
}

func (f *proxyForm) apply() {
	if f.path == "" {
		return
	}
	m, ok := f.parse()
	if !ok {
		return
	}
	if !f.writeEnv(&m) {
		return
	}
	f.applied = m
	f.picked = false
	if f.st.proxy.Method() != network.ProxyNone {
		f.setProxy(network.NoProxy())
	}
}

func (f *proxyForm) parse() (proxyenv.Manual, bool) {
	servers := make(map[proxyenv.Kind]proxyenv.Server, len(proxyenv.Kinds))
	ok := true
	for _, kind := range proxyenv.Kinds {
		in := f.servers[kind]
		host := strings.TrimSpace(in.host.Text())
		if host == "" {
			continue
		}
		s, err := proxyenv.ParseServer(host, uint16(in.port.Value()))
		if err != nil {
			in.field.SetError(proxyProblem(err))
			ok = false
			continue
		}
		servers[kind] = s
	}
	exclusions := make([]proxyenv.Exclusion, 0, len(f.exclude))
	for _, name := range f.chips.names() {
		x, err := proxyenv.ParseExclusion(name)
		if err != nil {
			f.showProblem(err)
			return proxyenv.Manual{}, false
		}
		exclusions = append(exclusions, x)
	}
	if !ok {
		return proxyenv.Manual{}, false
	}
	m, err := proxyenv.NewManual(servers, exclusions)
	if err != nil {
		f.showProblem(err)
		return proxyenv.Manual{}, false
	}
	f.problem.Clear()
	return m, true
}

func (f *proxyForm) showProblem(err error) { f.problem.Error(proxyProblem(err), err, nil) }

func (f *proxyForm) writeEnv(m *proxyenv.Manual) bool {
	if err := writeProxyEnv(f.wm, m); err != nil {
		f.showProblem(err)
		f.show(f.st)
		return false
	}
	return true
}

type proxyInConfigError struct{ names []string }

func (e *proxyInConfigError) Error() string {
	return fmt.Sprintf("settings: lys-wm's config file sets %s, which its runtime file cannot take out", strings.Join(e.names, ", "))
}

func writeProxyEnv(w *wmBackend, m *proxyenv.Manual) error {
	env, _ := w.value(wmEnvPath).(map[string]any)
	next := proxyenv.WithoutVariables(env)
	if m != nil {
		if next == nil {
			next = map[string]any{}
		}
		for name, v := range m.Env() {
			next[name] = v
		}
	}
	base, _ := w.baseValue(wmEnvPath).(map[string]any)
	kept := proxyenv.WithoutVariables(base)
	var stuck []string
	for name := range base {
		_, other := kept[name]
		_, written := next[name]
		if !other && !written {
			stuck = append(stuck, name)
		}
	}
	if len(stuck) > 0 {
		slices.Sort(stuck)
		return &proxyInConfigError{names: stuck}
	}
	if maps.EqualFunc(env, next, func(a, b any) bool { return reflect.DeepEqual(a, b) }) {
		return nil
	}
	if len(next) == 0 {
		return w.edit(wmconfig.Edit{Path: wmPath(wmEnvPath), Unset: true})
	}
	return w.edit(wmconfig.Edit{Path: wmPath(wmEnvPath), Value: next})
}
