package pauser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"

	"github.com/stubbedev/lys/lib/busauth"
	"github.com/stubbedev/lys/lib/polkit"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/statefile"
	"github.com/stubbedev/lys/power/pause"
)

const SystemAction polkit.Action = "dev.stubbe.lys.power.pause-system-units"

type Authorizer = busauth.Authorizer

func Polkit(conn *dbus.Conn) Authorizer { return busauth.Polkit(conn, SystemAction) }

type stored struct {
	Picks   []pause.Unit `json:"picks"`
	Paused  []pause.Unit `json:"paused"`
	Resumed bool         `json:"resumed"`
}

type Pauser struct {
	path      string
	managers  Managers
	authorize Authorizer
	log       *slog.Logger
	wakes     chan struct{}

	mu      sync.Mutex
	server  *pause.Server
	stored  stored
	saved   []byte
	entries []pause.Entry
}

func Load(path string, managers Managers, authorize Authorizer, log *slog.Logger) (*Pauser, error) {
	p := &Pauser{path: path, managers: managers, authorize: authorize, log: log, wakes: make(chan struct{}, 1)}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		p.saved, err = json.Marshal(p.stored)
		return p, err
	}
	if err != nil {
		return nil, fmt.Errorf("read the paused units from %s: %w", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p.stored); err != nil {
		return nil, fmt.Errorf("read the paused units from %s: %w", path, err)
	}
	p.saved = data
	p.entries = pausedEntries(p.stored.Paused, nil)
	return p, nil
}

func (p *Pauser) Serve(conn *dbus.Conn) (func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	server, release, err := pause.Serve(conn, p, p.state())
	if err != nil {
		return nil, err
	}
	p.server = server
	return release, nil
}

func (p *Pauser) Wakes() <-chan struct{} { return p.wakes }

func (p *Pauser) wake() {
	select {
	case p.wakes <- struct{}{}:
	default:
	}
}

func (p *Pauser) Apply(ctx context.Context, onBattery bool) (failing bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !onBattery {
		p.stored.Resumed = false
	}
	holding := Holding(onBattery, p.stored.Resumed)
	conns := &connections{ctx: ctx, managers: p.managers, users: map[uint32]userManager{}}
	defer conns.close(p.log)

	var failures []pause.Entry
	running := map[pause.Unit]bool{}
	for _, u := range Candidates(p.stored.Picks, p.stored.Paused, holding) {
		m, err := conns.of(u)
		if errors.Is(err, ErrNoManager) {
			continue
		}
		if err == nil {
			running[u], err = m.Running(ctx, u.Name())
		}
		if err != nil {
			failures = append(failures, pause.Entry{Unit: u, Status: pause.StatusPauseFailed, Err: err.Error()})
		}
	}
	plan := Decide(p.stored.Picks, p.stored.Paused, holding, running)

	paused := slices.Clone(p.stored.Paused)
	for _, u := range plan.Resume {
		err := conns.do(u, resumeUnit)
		switch {
		case errors.Is(err, ErrNoManager):
			p.log.Info("dropping a paused unit whose manager is gone, and its freezer with it", "unit", u, "err", err)
		case err != nil:
			failures = append(failures, pause.Entry{Unit: u, Status: pause.StatusResumeFailed, Err: err.Error()})
			continue
		default:
			p.log.Info("resumed", "unit", u)
		}
		paused = slices.DeleteFunc(paused, func(v pause.Unit) bool { return v == u })
	}
	for _, u := range plan.Pause {
		if err := conns.do(u, pauseUnit); err != nil {
			failures = append(failures, pause.Entry{Unit: u, Status: pause.StatusPauseFailed, Err: err.Error()})
			continue
		}
		p.log.Info("paused on battery", "unit", u)
		paused = append(paused, u)
	}
	for _, f := range failures {
		p.log.Error("pausing on battery", "unit", f.Unit, "status", f.Status, "err", f.Err)
	}
	p.stored.Paused = pause.Sorted(paused)
	p.entries = pausedEntries(p.stored.Paused, failures)
	err = p.save()
	p.publish()
	return len(failures) > 0, err
}

func pausedEntries(paused []pause.Unit, failures []pause.Entry) []pause.Entry {
	entries := slices.Clone(failures)
	for _, u := range paused {
		if !slices.ContainsFunc(failures, func(f pause.Entry) bool { return f.Unit == u }) {
			entries = append(entries, pause.Entry{Unit: u, Status: pause.StatusPaused})
		}
	}
	slices.SortFunc(entries, func(a, b pause.Entry) int { return pause.Compare(a.Unit, b.Unit) })
	return entries
}

func (p *Pauser) save() error {
	data, err := json.Marshal(p.stored)
	if err != nil {
		return fmt.Errorf("encode the paused units: %w", err)
	}
	if bytes.Equal(data, p.saved) {
		return nil
	}
	if err := statefile.Write(p.path, data); err != nil {
		return fmt.Errorf("record the paused units: %w", err)
	}
	p.saved = data
	return nil
}

func (p *Pauser) state() pause.State {
	return pause.State{Picks: p.stored.Picks, Entries: p.entries, Resumed: p.stored.Resumed}
}

func (p *Pauser) publish() {
	if p.server == nil {
		return
	}
	if err := p.server.Publish(p.state()); err != nil {
		p.log.Error("publishing the paused units", "err", err)
	}
}

func (p *Pauser) change(edit func(*stored)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	edit(&p.stored)
	err := p.save()
	p.publish()
	p.wake()
	return err
}

func (p *Pauser) PickSystem(ctx context.Context, c pause.Caller, units []pause.Unit) error {
	if c.UID != 0 {
		if err := p.authorize(ctx, c); err != nil {
			return err
		}
	}
	return p.change(func(s *stored) {
		s.Picks = replaced(s.Picks, units, func(u pause.Unit) bool { return u.Scope() == pause.ScopeSystem })
	})
}

func (p *Pauser) PickUser(_ context.Context, c pause.Caller, units []pause.Unit) error {
	return p.change(func(s *stored) {
		s.Picks = replaced(s.Picks, units, func(u pause.Unit) bool { return u.Scope() == pause.ScopeUser && u.UID() == c.UID })
	})
}

func (p *Pauser) Resume(context.Context, pause.Caller) error {
	return p.change(func(s *stored) { s.Resumed = true })
}

func replaced(picks, with []pause.Unit, mine func(pause.Unit) bool) []pause.Unit {
	return pause.Sorted(append(slices.DeleteFunc(slices.Clone(picks), mine), with...))
}

type connections struct {
	ctx      context.Context
	managers Managers
	users    map[uint32]userManager
	closers  []func() error
}

type userManager struct {
	m   Manager
	err error
}

func (c *connections) of(u pause.Unit) (Manager, error) {
	if u.Scope() == pause.ScopeSystem {
		return c.managers.System(), nil
	}
	if um, ok := c.users[u.UID()]; ok {
		return um.m, um.err
	}
	m, closer, err := c.managers.User(c.ctx, u.UID())
	c.users[u.UID()] = userManager{m, err}
	if err != nil {
		return nil, err
	}
	c.closers = append(c.closers, closer)
	return m, nil
}

func (c *connections) do(u pause.Unit, act func(context.Context, Manager, pause.Unit) error) error {
	m, err := c.of(u)
	if err != nil {
		return err
	}
	return act(c.ctx, m, u)
}

func (c *connections) close(log *slog.Logger) {
	for _, closer := range c.closers {
		if err := closer(); err != nil {
			log.Warn("closing a user's systemd manager connection", "err", err)
		}
	}
}
