package appnap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/nap"
)

const wakeTimeout = 5 * time.Second

// Applier makes one change.
type Applier interface {
	Apply(ctx context.Context, c Change) error
}

// Napper remembers what it applied, so each plan costs only its changes.
type Napper struct {
	apply   Applier
	applied map[nap.Unit]nap.Treatment
}

// NewNapper starts with every app awake.
func NewNapper(a Applier) *Napper {
	return &Napper{apply: a, applied: map[nap.Unit]nap.Treatment{}}
}

// Reconcile applies the changes from the applied treatments to next.
func (n *Napper) Reconcile(ctx context.Context, next map[nap.Unit]nap.Treatment) error {
	var errs error
	for _, c := range Changes(n.applied, next) {
		errs = errors.Join(errs, n.apply.Apply(ctx, c))
	}
	clear(n.applied)
	for u, t := range next {
		if t != nap.Awake {
			n.applied[u] = t
		}
	}
	return errs
}

// WakeAll restores every app it napped.
func (n *Napper) WakeAll(ctx context.Context) error {
	return n.Reconcile(ctx, nil)
}

// Sources read the session's state; Proc is where /proc is.
type Sources struct {
	Windows   func() []wmipc.Window
	Streams   func() []int32
	Holds     func(ctx context.Context) ([]Hold, error)
	OnBattery func(ctx context.Context) (bool, error)
	Levels    func() (map[string]nap.Level, error)
	Proc      string
}

// Inputs reads every source once.
func (s Sources) Inputs(ctx context.Context) (Inputs, error) {
	levels, err := s.Levels()
	if err != nil {
		return Inputs{}, fmt.Errorf("the nap levels: %w", err)
	}
	in := Inputs{Windows: s.Windows(), Streams: s.Streams(), Levels: levels, Supply: nap.SupplyMains}
	holds, err := s.Holds(ctx)
	if err != nil {
		return Inputs{}, fmt.Errorf("the idle inhibitors' processes: %w", err)
	}
	in.Holds = holds
	onBattery, err := s.OnBattery(ctx)
	if err != nil {
		return Inputs{}, fmt.Errorf("the power supply: %w", err)
	}
	if onBattery {
		in.Supply = nap.SupplyBattery
	}
	pids := make([]int32, 0, len(in.Windows)+len(in.Streams)+len(in.Holds))
	for _, w := range in.Windows {
		pids = append(pids, w.PID)
	}
	pids = append(pids, in.Streams...)
	for _, h := range in.Holds {
		pids = append(pids, h.PID)
	}
	in.Units, err = UnitsOf(s.Proc, pids)
	return in, err
}

// Run plans and applies on every tick until ctx ends or ticks close,
// then wakes every app it napped.
func Run(ctx context.Context, n *Napper, src Sources, ticks <-chan struct{}, report func(error)) {
	defer func() {
		wake, cancel := context.WithTimeout(context.WithoutCancel(ctx), wakeTimeout)
		defer cancel()
		if err := n.WakeAll(wake); err != nil {
			report(fmt.Errorf("wake the napping apps: %w", err))
		}
	}()
	for {
		in, err := src.Inputs(ctx)
		if err != nil {
			report(err)
		}
		if err == nil || in.Units != nil {
			if err := n.Reconcile(ctx, Plan(in)); err != nil {
				report(err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
		}
	}
}
