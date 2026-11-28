package settings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

// maxSweepActions bounds one phase of a row's sweep, far above the
// busiest row's controls, so a phase that never ends fails.
const maxSweepActions = 5000

// sweepPhase is which of a row's controls a sweep step drives.
type sweepPhase int

const (
	phaseAdd sweepPhase = iota
	phaseEdit
	phaseRemove
	phaseReset
)

// sweepAction is one step of a sweep. Its name is what the step reports
// as, made only for a step taken, since a sweep reads every action of a
// row after each step it takes.
type sweepAction struct {
	phase sweepPhase
	name  func() string
	do    func()
	drove string
}

func shownAndEnabled(w widget.Widget) bool {
	v, ok := w.(interface{ Visible() bool })
	return ok && v.Visible() && widget.IsEnabled(w)
}

func buttonPhase(b *widget.Button) sweepPhase {
	role, ok := roleOf(b)
	switch {
	case !ok:
		return phaseEdit
	case role == roleAdd, role == roleListAdd:
		return phaseAdd
	case role == roleReset:
		return phaseReset
	case role == roleRemove:
		return phaseRemove
	}
	return phaseEdit
}

// sweepActions is everything a user can do to the row's controls
// without typing, in tree order: every button, switch, check, segment,
// dropdown row, spin step and clear, slider end and colour pick. A
// dropdown goes through only its first and last row where its rows are in
// driven or an earlier dropdown in the tree shows the same rows; the
// action picking the last of a full run names the rows it drove.
func sweepActions(root widget.Widget, driven map[string]bool, kinds embeddings) []sweepAction {
	var out []sweepAction
	edit := func(name string, do func()) {
		out = append(out, sweepAction{phase: phaseEdit, name: func() string { return name }, do: do})
	}
	inTree := map[string]bool{}
	var walk func(w widget.Widget)
	walk = func(w widget.Widget) {
		if !shownAndEnabled(w) {
			return
		}
		if s, ok := w.(*slider); ok {
			for _, end := range []struct {
				name string
				past float64
			}{{"end", math.MaxFloat64}, {"start", -math.MaxFloat64}} {
				edit("slider to its "+end.name, func() {
					s.Set(end.past)
					s.OnCommit(s.Knob.Value())
				})
			}
			return
		}
		switch x := kinds.embedded(w).(type) {
		case *widget.ColorButton:
			edit("pick a colour", func() { x.Choose(render.RGBA(0x33, 0x66, 0x99, 0xff)) })
			return
		case *widget.Button:
			if x.OnClick != nil {
				out = append(out, sweepAction{phase: buttonPhase(x), name: func() string { return fmt.Sprintf("click %q %v", x.TooltipText(), x.Classes()) }, do: x.OnClick})
			}
		case *widget.Switch:
			edit("toggle", x.Toggle)
		case *widget.CheckButton:
			edit("check", x.Toggle)
		case *widget.ToggleButton:
			edit("toggle the button", func() { x.SetActive(!x.Active()) })
		case *widget.ChoiceChip:
			edit("press the chip", func() { x.ClickAt(widget.Point{}) })
			return
		case *widget.ToggleGroup:
			for i := range x.Len() {
				edit(fmt.Sprintf("segment %d", i), func() { x.SetActive(i) })
			}
		case *widget.Dropdown:
			items := x.Items()
			key := strings.Join(items, "\x00")
			full := !driven[key] && !inTree[key]
			inTree[key] = true
			for i := range items {
				if !full && i != 0 && i != len(items)-1 {
					continue
				}
				a := sweepAction{phase: phaseEdit, name: func() string { return fmt.Sprintf("dropdown row %d (%s)", i, items[i]) }, do: func() {
					x.SetSelected(i)
					if x.OnSelect != nil {
						x.OnSelect(i)
					}
				}}
				if full && i == len(items)-1 {
					a.drove = key
				}
				out = append(out, a)
			}
			return
		case *widget.SpinButton:
			edit("spin up", func() { x.KeyAction(widget.KeyUp, 0) })
			edit("spin down", func() { x.KeyAction(widget.KeyDown, 0) })
			for _, end := range []struct {
				name string
				at   func(lo, hi float64) float64
			}{{"least", func(lo, _ float64) float64 { return lo }}, {"most", func(_, hi float64) float64 { return hi }}} {
				edit("spin to its "+end.name, func() {
					x.SetValue(end.at(x.Range()))
					if x.OnValueChanged != nil {
						x.OnValueChanged(x.Value())
					}
				})
			}
			if x.OnCleared != nil {
				edit("clear the spin", func() { x.Clear(); x.OnCleared() })
			}
		case *widget.Slider:
			edit("slider to its end", func() { x.KeyAction(widget.KeyEnd, 0) })
			edit("slider to its start", func() { x.KeyAction(widget.KeyHome, 0) })
		}
		if p, ok := w.(interface{ Children() []widget.Widget }); ok {
			for _, c := range p.Children() {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// sweepRow drives the row through its phases: each add there is at the
// start, then every edit, then every remove, then the reset; after runs
// after each step. Steps are taken by position, re-read after each, since
// a step may rebuild the controls. The dropdown rows it drives all of join
// driven once the row is done.
func sweepRow(t *testing.T, row *fieldRow, driven map[string]bool, after func(action string)) {
	t.Helper()
	met := map[string]bool{}
	defer maps.Copy(driven, met)
	popovers := &fakePickers{}
	row.k.pickers = popovers
	kinds := embeddings{}
	step := func(a sweepAction) {
		name := a.name()
		popovers.content = nil
		a.do()
		if a.drove != "" {
			met[a.drove] = true
		}
		row.refresh()
		after(name)
		if popovers.content == nil {
			return
		}
		for _, in := range sweepActions(popovers.content, driven, kinds) {
			if in.phase == phaseEdit {
				inName := in.name()
				in.do()
				row.refresh()
				after(name + ", then " + inName + " in what it opened")
				return
			}
		}
	}
	of := func(phase sweepPhase) []sweepAction {
		var out []sweepAction
		for _, a := range sweepActions(row, driven, kinds) {
			if a.phase == phase {
				out = append(out, a)
			}
		}
		return out
	}
	for i := range len(of(phaseAdd)) {
		if actions := of(phaseAdd); i < len(actions) {
			step(actions[i])
		}
	}
	for i := range maxSweepActions + 1 {
		actions := of(phaseEdit)
		if i >= len(actions) {
			break
		}
		if i == maxSweepActions {
			t.Fatalf("%s: more than %d steps", row.path, maxSweepActions)
		}
		step(actions[i])
	}
	for range maxSweepActions {
		removes := of(phaseRemove)
		if len(removes) == 0 {
			break
		}
		step(removes[0])
		if len(of(phaseRemove)) >= len(removes) {
			break
		}
	}
	for _, a := range of(phaseReset) {
		step(a)
	}
}

// Every lys-wm control, driven through every add, edit, remove and reset,
// writes only what lys-wm's own --check-config takes: each write goes
// through it before it lands, and a refusal fails the test. Each list of
// dropdown rows goes through every row once, the first time a control
// shows it.
func TestEveryWMControlWritesOnlyWhatLysWMAccepts(t *testing.T) {
	if os.Getenv("LYS_SETTINGS_HEADLESS") == "" {
		t.Skip("needs the dev shell's lys-wm; set by `just test`")
	}
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeFile(t, filepath.Join(dir, "lys-wm", "config.yaml"), sweepWMConfig)
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	driven := map[string]bool{}
	for _, spec := range allWMRows() {
		row := newFieldRow(k, w, spec)
		k.onWMChange = row.refresh
		sweepRow(t, row, driven, func(action string) {
			if w.notice.failed() {
				t.Errorf("%s: %s: %v", spec.path, action, w.notice.err)
				w.show(wmNotice{})
			}
		})
	}
}

// Every lys-shell control, driven through every add, edit, remove and
// reset, writes only values lys-shell's validator takes, and what it saved
// loads back without an error: each write is checked as it is made, the
// saved file once the row's sweep is done. Each list of dropdown rows goes
// through every row once, the first time a control shows it.
func TestEveryShellControlWritesOnlyWhatLysShellAccepts(t *testing.T) {
	base := testKit(t, "")
	driven := map[string]bool{}
	for _, spec := range shellRows() {
		t.Run(spec.path, func(t *testing.T) {
			k := *base
			k.store = testStore(t, "")
			row := newFieldRow(&k, k.store, spec)
			var steps []string
			sweepRow(t, row, driven, func(action string) {
				steps = append(steps, action)
				if problemShown(row.settingRow) {
					t.Errorf("%s: refused: %s", action, problemText(row.settingRow))
					row.showProblem(rowProblem{})
				}
			})
			var problems []string
			svc := config.Load(k.store.svc.Dir(), func(d config.Diagnostic) {
				if d.Kind == config.DiagnosticError {
					problems = append(problems, d.Plain())
				}
			})
			svc.Close()
			if len(problems) > 0 {
				t.Errorf("after %q lys-shell reads back %v\nruntime.yaml:\n%s", steps, problems, runtimeFile(t, &k))
			}
		})
	}
}

// refusingBackend is a backend whose every write fails with err.
type refusingBackend struct {
	backend
	err error
}

func (b refusingBackend) slot(path string) slot {
	s := b.backend.slot(path)
	s.set = func(any) error { return b.err }
	return s
}

// Every lys-shell row whose write is refused says so in the app's
// words: the row's problem line is a summary over the refusal's error,
// which waits in its closed detail, and no shown text but an opened
// detail reads as an error chain. Each list of dropdown rows goes through
// every row once, the first time a control shows it.
func TestEveryShellRowSaysARefusedWriteInTheAppsWords(t *testing.T) {
	base := testKit(t, "")
	driven := map[string]bool{}
	summary := locale.T().Get("settings-row-not-saved")
	refusals := 0
	for _, spec := range shellRows() {
		t.Run(spec.path, func(t *testing.T) {
			k := *base
			k.store = testStore(t, "")
			err := actionFailure("write " + spec.path)
			row := newFieldRow(&k, refusingBackend{k.store, err}, spec)
			sweepRow(t, row, driven, func(action string) {
				if !problemShown(row.settingRow) {
					return
				}
				p := row.problem
				refusals++
				if p.kind != stateError || p.Text() != summary || p.detail.Text() != err.Error() || p.detail.Visible() {
					t.Errorf("%s: the row says %v %q over %q, detail shown %v", action, p.kind, p.Text(), p.detail.Text(), p.detail.Visible())
				}
				walkShown(row, func(w widget.Widget) {
					if l, ok := w.(*widget.Label); ok && l.Visible() && !widget.HasClass(l, "card-state-detail") && rawError(l.Text()) {
						t.Errorf("%s: the row reads %q", action, l.Text())
					}
				})
				p.Clear()
			})
		})
	}
	if refusals == 0 {
		t.Fatal("no row said a refused write")
	}
}

// sweepWMConfig holds one item of every lys-wm list, so a sweep opens
// and edits each kind of card once instead of every built-in bind.
const sweepWMConfig = `binds:
  - {chord: SUPER + Q, action: kill}
submaps:
  resize:
    - {chord: Return, action: submap reset}
gestures:
  swipe:
    - {fingers: 3, direction: horizontal, drive: workspaces}
  pinch:
    - {fingers: 2, direction: both, drive: zoom}
  hold:
    - {fingers: 4, action: kill}
outputs:
  - {match: DP-1, scale: 1}
rules:
  - {app_id: foot, float: true}
autostart:
  - foot
env:
  GDK_BACKEND: wayland
palette:
  mine: "#123456"
`

// embeddings remembers, per type, whether a control wraps a lys-ui
// widget by embedding it as its first field, so a sweep reading a row's
// controls after every step reflects on each type once.
type embeddings map[reflect.Type]bool

// embedded is the lys-ui widget a control wraps by embedding it (a
// number's spin, a toggle's switch), else the widget itself.
func (e embeddings) embedded(w widget.Widget) widget.Widget {
	for {
		t := reflect.TypeOf(w)
		if t == nil {
			return w
		}
		wraps, known := e[t]
		if !known {
			wraps = wrapsAWidget(t)
			e[t] = wraps
		}
		v := reflect.ValueOf(w)
		if !wraps || v.IsNil() {
			return w
		}
		f := v.Elem().Field(0)
		if f.IsNil() {
			return w
		}
		w = f.Interface().(widget.Widget)
	}
}

// wrapsAWidget reports a pointer to a struct outside lys-ui whose first
// field embeds a pointer to a widget it can hand out.
func wrapsAWidget(t reflect.Type) bool {
	if t.Kind() != reflect.Pointer || t.Elem().Kind() != reflect.Struct || t.Elem().PkgPath() == reflect.TypeFor[widget.Button]().PkgPath() || t.Elem().NumField() == 0 {
		return false
	}
	f := t.Elem().Field(0)
	return f.Anonymous && f.IsExported() && f.Type.Kind() == reflect.Pointer && f.Type.Implements(reflect.TypeFor[widget.Widget]())
}

// controlState is what the row's controls show: every shown switch,
// check, segment, dropdown, spin and entry, in tree order.
func controlState(root widget.Widget) []string {
	var out []string
	kinds := embeddings{}
	var walk func(w widget.Widget)
	walk = func(w widget.Widget) {
		if v, ok := w.(interface{ Visible() bool }); !ok || !v.Visible() {
			return
		}
		switch x := kinds.embedded(w).(type) {
		case *widget.Switch:
			out = append(out, fmt.Sprintf("switch %v", x.On()))
		case *widget.CheckButton:
			out = append(out, fmt.Sprintf("check %v", x.Checked()))
		case *widget.ToggleGroup:
			out = append(out, fmt.Sprintf("segment %d", x.Active()))
		case *widget.ChoiceChips:
			out = append(out, fmt.Sprintf("chips %v", x.Checked()))
		case *widget.Dropdown:
			out = append(out, fmt.Sprintf("dropdown %d", x.Selected()))
			return
		case *widget.SpinButton:
			if x.Empty() {
				out = append(out, "spin empty")
				return
			}
			out = append(out, fmt.Sprintf("spin %v", x.Value()))
			return
		case *widget.Entry:
			out = append(out, "entry "+x.Text())
			return
		}
		if p, ok := w.(interface{ Children() []widget.Widget }); ok {
			for _, c := range p.Children() {
				walk(c)
			}
		}
	}
	walk(root)
	return out
}

// Every lys-wm control, when lys-wm refuses what it wrote, shows again
// what a row built from the stored config shows: the last accepted state.
// Each list of dropdown rows goes through every row once, the first time
// a control shows it.
func TestARefusedChangeLeavesEveryWMControlAtTheLastAcceptedState(t *testing.T) {
	if os.Getenv("LYS_SETTINGS_HEADLESS") == "" {
		t.Skip("needs the dev shell's lys-wm; set by `just test`")
	}
	k := testKit(t, "")
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	writeFile(t, filepath.Join(dir, "lys-wm", "config.yaml"), sweepWMConfig)
	refusing := func(ctx context.Context, argv ...string) ([]byte, error) {
		if len(argv) > 1 && argv[1] == "--check-config" {
			return nil, &wmconfig.ToolError{Argv: argv, Exit: 1, Stderr: "refused by the test", Err: errors.New("exit status 1")}
		}
		return wmconfig.Exec(ctx, argv...)
	}
	w, err := openWM(k, refusing)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	driven := map[string]bool{}
	for _, spec := range allWMRows() {
		row := newFieldRow(k, w, spec)
		k.onWMChange = row.refresh
		sweepRow(t, row, driven, func(action string) {
			if !w.notice.failed() {
				return
			}
			notice := w.notice.err
			w.show(wmNotice{})
			fresh := newFieldRow(k, w, spec)
			if got, want := controlState(row), controlState(fresh); !slices.Equal(got, want) {
				t.Errorf("%s (%s): after a refused %s (%s) the row shows %q, the stored config %q", spec.path, spec.labelKey(), action, notice, got, want)
			}
		})
		noFile(t, w.file.Runtime())
	}
}
