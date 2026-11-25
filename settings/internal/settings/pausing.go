package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/pause"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const pauseCallWait = pause.AuthTimeout + 10*time.Second

type pauseCandidate struct {
	unit        pause.Unit
	description string
}

type pauseListing struct {
	state      pause.State
	candidates []pauseCandidate
	err        error
}

type pauseControl interface {
	state(ctx context.Context) (pause.State, error)
	candidates(ctx context.Context) ([]pauseCandidate, error)
	pickSystem(ctx context.Context, names []string) error
	pickUser(ctx context.Context, names []string) error
	close() error
}

type busPausing struct {
	system, session *dbus.Conn
	client          *pause.Client
	uid             uint32
}

func openPausing(system, session func() (*dbus.Conn, error)) (pauseControl, error) {
	sys, err := system()
	if err != nil {
		return nil, fmt.Errorf("connect to the system bus: %w", err)
	}
	sess, err := session()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("connect to the session bus: %w", err), sys.Close())
	}
	return &busPausing{system: sys, session: sess, client: pause.NewClient(sys), uid: intconv.Must[uint32](os.Getuid())}, nil
}

func (b *busPausing) state(ctx context.Context) (pause.State, error) { return b.client.State(ctx) }

func (b *busPausing) candidates(ctx context.Context) ([]pauseCandidate, error) {
	system, err := listUnits(ctx, b.system)
	if err != nil {
		return nil, fmt.Errorf("list the system's units: %w", err)
	}
	user, err := listUnits(ctx, b.session)
	if err != nil {
		return nil, fmt.Errorf("list your units: %w", err)
	}
	return pausable(system, user, b.uid), nil
}

func (b *busPausing) pickSystem(ctx context.Context, names []string) error {
	return b.client.PickSystem(ctx, names)
}

func (b *busPausing) pickUser(ctx context.Context, names []string) error {
	return b.client.PickUser(ctx, names)
}

func (b *busPausing) close() error { return errors.Join(b.system.Close(), b.session.Close()) }

func listUnits(ctx context.Context, conn *dbus.Conn) ([]listedUnit, error) {
	var listed []listedUnit
	if err := conn.Object(systemdName, systemdPath).CallWithContext(ctx, systemdManager+".ListUnits", 0).Store(&listed); err != nil {
		return nil, fmt.Errorf("ListUnits: %w", err)
	}
	return listed, nil
}

func pausable(system, user []listedUnit, uid uint32) []pauseCandidate {
	var out []pauseCandidate
	add := func(listed []listedUnit, unit func(string) (pause.Unit, error)) {
		for _, l := range listed {
			if l.Load != "loaded" {
				continue
			}
			if u, err := unit(l.Name); err == nil {
				out = append(out, pauseCandidate{unit: u, description: l.Description})
			}
		}
	}
	add(system, pause.System)
	add(user, func(name string) (pause.Unit, error) { return pause.User(uid, name) })
	slices.SortFunc(out, func(a, b pauseCandidate) int { return pause.Compare(a.unit, b.unit) })
	return slices.CompactFunc(out, func(a, b pauseCandidate) bool { return a.unit == b.unit })
}

func picksOf(st pause.State, scope pause.Scope, uid uint32) []string {
	var names []string
	for _, u := range st.Picks {
		if u.Scope() == scope && u.VisibleTo(uid) {
			names = append(names, u.Name())
		}
	}
	return names
}

func withPick(st pause.State, u pause.Unit, on bool) []string {
	names := slices.DeleteFunc(picksOf(st, u.Scope(), u.UID()), func(n string) bool { return n == u.Name() })
	if on {
		names = append(names, u.Name())
	}
	slices.Sort(names)
	return names
}

type pausing struct {
	*widget.Box
	k       *kit
	uid     uint32
	listing pauseListing
	problem *inlineState
	rows    *widget.ItemGroup
	state   *cardState
	add     *widget.Button
	addRow  *insetRow
}

func newPausing(k *kit) *pausing {
	s := &pausing{k: k, uid: intconv.Must[uint32](os.Getuid()), rows: widget.NewItemGroup(), state: k.cardState()}
	body := gapBox(widget.Column)
	s.problem = k.inlineState()
	body.Append(s.problem, false)
	body.Append(s.rows, false)
	s.add = k.listAddButton("settings-power-pause-add", s.openPicker)
	s.addRow = newInsetAction(s.add)
	body.Append(s.addRow, false)
	s.Box = newSection(k, "settings-power-section-pause", body)
	s.load()
	return s
}

func (s *pausing) load() {
	async(s.k, func() pauseListing {
		ctl, err := s.k.pausing()
		if err != nil {
			return pauseListing{err: err}
		}
		listing := pauseListing{}
		listing.state, listing.err = callWithin(powerCallWait, ctl.state)
		if listing.err == nil {
			listing.candidates, listing.err = callWithin(powerCallWait, ctl.candidates)
		}
		listing.err = errors.Join(listing.err, ctl.close())
		return listing
	}, s.show)
}

func (s *pausing) fail(err error) {
	s.problem.Result(locale.T().Get("settings-power-pause-refused"), err)
}

func (s *pausing) show(l pauseListing) {
	t := locale.T()
	s.listing = l
	s.rows.Clear()
	s.rows.Append(s.state)
	s.addRow.SetVisible(l.err == nil)
	if l.err != nil {
		s.state.Unavailable("ld-battery-full-symbolic", t.Get("settings-power-pause-unavailable"), l.err)
		return
	}
	st := l.state.VisibleTo(s.uid)
	units := slices.Clone(st.Picks)
	for _, e := range st.Entries {
		units = append(units, e.Unit)
	}
	units = pause.Sorted(units)
	if len(units) == 0 {
		s.state.Empty("ld-battery-full-symbolic", t.Get("settings-power-pause-none"))
		return
	}
	s.state.Clear()
	for _, u := range units {
		s.rows.Append(s.row(st, u))
	}
}

func scopeText(u pause.Unit) string {
	if u.Scope() == pause.ScopeSystem {
		return locale.T().Get("settings-power-pause-system")
	}
	return locale.T().Get("settings-power-pause-user")
}

// entryText is u's scope, and that it is paused while it is; failed
// is u's pause or resume that failed, over the error lys-power got,
// none while neither did.
func entryText(st pause.State, u pause.Unit) (text string, failed rowProblem) {
	t := locale.T()
	text = scopeText(u)
	i := slices.IndexFunc(st.Entries, func(e pause.Entry) bool { return e.Unit == u })
	if i < 0 {
		return text, rowProblem{}
	}
	e := st.Entries[i]
	switch e.Status {
	case pause.StatusPaused:
		return text + " · " + t.Get("settings-power-pause-paused"), rowProblem{}
	case pause.StatusPauseFailed:
		return text, rowProblem{summary: t.Get("settings-power-pause-failed"), err: errors.New(e.Err)}
	case pause.StatusResumeFailed:
		return text, rowProblem{summary: t.Get("settings-power-pause-resume-failed"), err: errors.New(e.Err)}
	}
	return text, rowProblem{}
}

func (s *pausing) row(st pause.State, u pause.Unit) widget.Widget {
	name := u.Name()
	if i := slices.IndexFunc(s.listing.candidates, func(c pauseCandidate) bool { return c.unit == u }); i >= 0 {
		name = cmp.Or(s.listing.candidates[i].description, name)
	}
	text, failed := entryText(st, u)
	row := s.k.row(name, u.Name()+" · "+text)
	row.AddClass("pause-unit")
	row.showProblem(failed)
	if slices.Contains(st.Picks, u) {
		row.AddAction(s.k.removeButton("settings-power-pause-remove", func() {
			s.set(u, false)
			s.k.offerUndo(name, func() { s.set(u, true) })
		}))
	}
	return row
}

func (s *pausing) set(u pause.Unit, on bool) {
	names := withPick(s.listing.state, u, on)
	async(s.k, func() error {
		ctl, err := s.k.pausing()
		if err != nil {
			return err
		}
		err = doWithin(pauseCallWait, func(ctx context.Context) error {
			if u.Scope() == pause.ScopeSystem {
				return ctl.pickSystem(ctx, names)
			}
			return ctl.pickUser(ctx, names)
		})
		return errors.Join(err, ctl.close())
	}, func(err error) {
		s.fail(err)
		s.load()
	})
}

func (s *pausing) offered() []pauseCandidate {
	return slices.DeleteFunc(slices.Clone(s.listing.candidates), func(c pauseCandidate) bool {
		return slices.Contains(s.listing.state.Picks, c.unit)
	})
}

func (s *pausing) openPicker() {
	offered := s.offered()
	keys := make([]string, len(offered))
	byKey := make(map[string]pauseCandidate, len(offered))
	for i, c := range offered {
		keys[i] = c.unit.String()
		byKey[keys[i]] = c
	}
	text := func(key string) string {
		c := byKey[key]
		return cmp.Or(c.description, c.unit.Name()) + " · " + c.unit.Name() + " · " + scopeText(c.unit)
	}
	openSearchPicker(s.k, s.add, keys, pickerSpec{
		class: "choice-popover", placeholder: "settings-power-pause-search", maxH: choiceListPx,
		row:  func(k *kit, key string) widget.Widget { return k.label(text(key)) },
		text: text,
	}, func(key string) { s.set(byKey[key].unit, true) })
}
