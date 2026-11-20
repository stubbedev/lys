package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const shellCallWait = 5 * time.Second

// switchControl is a lys-shell state with an on/off switch: read reads
// it, changes ticks whenever it may have changed, set switches it.
type switchControl[S any] interface {
	read(ctx context.Context) (S, error)
	changes(ctx context.Context) (<-chan struct{}, error)
	set(ctx context.Context, on bool) error
	close() error
}

// switchSpec is what a shellSwitch shows: the state opened from lys-shell,
// whether it is on and the line saying it, and the message keys of its
// row, its waiting, its absence and a refused switch.
type switchSpec[S any] struct {
	open                                 func(k *kit) (switchControl[S], error)
	on                                   func(S) bool
	words                                func(k *kit, s S) string
	section, label, loading, unavailable string
	refused                              string
}

// shellSwitch is a row switching a lys-shell state as it stands, its
// status line following every change lys-shell announces.
type shellSwitch[S any] struct {
	*widget.Box
	k       *kit
	spec    switchSpec[S]
	ctl     switchControl[S]
	stop    context.CancelFunc
	toggle  *widget.Switch
	status  *widget.Label
	problem *widget.Label
	state   S
	known   bool
	syncing bool
	unread  bool
	closed  bool
}

func newShellSwitch[S any](k *kit, spec switchSpec[S]) *shellSwitch[S] {
	t := locale.T()
	p := &shellSwitch[S]{Box: gapBox(widget.Column), k: k, spec: spec, toggle: widget.NewSwitch(false)}
	p.toggle.SetAccessibleName(t.Get(spec.label))
	p.toggle.SetEnabled(false)
	p.toggle.OnChanged = p.switched
	p.status = detailLabel(k, t.Get(spec.loading))
	p.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	p.problem.SetVisible(false)
	rows := widget.NewItemGroup()
	rows.Append(panelRowWith(k, t.Get(spec.label), p.status, p.toggle))
	if spec.section != "" {
		p.Append(newSection(k, spec.section, rows), false)
	} else {
		p.Append(rows, false)
	}
	p.Append(p.problem, false)
	type opened struct {
		ctl   switchControl[S]
		state S
		err   error
	}
	async(k, func() opened {
		ctl, err := spec.open(k)
		if err != nil {
			return opened{err: err}
		}
		st, err := callWithin(shellCallWait, ctl.read)
		if err != nil {
			return opened{err: errors.Join(err, ctl.close())}
		}
		return opened{ctl: ctl, state: st}
	}, func(o opened) {
		if o.err != nil {
			p.status.SetText(t.Get(spec.unavailable, i18n.Str("reason", o.err.Error())))
			return
		}
		if p.closed {
			_ = o.ctl.close()
			return
		}
		p.ctl = o.ctl
		p.follow()
		p.show(o.state)
	})
	return p
}

func (p *shellSwitch[S]) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	ticks, err := p.ctl.changes(ctx)
	if err != nil {
		cancel()
		p.fail(err)
		return
	}
	p.stop = cancel
	followTicks(p.k, ticks, p.reload)
}

func (p *shellSwitch[S]) reload() {
	if p.closed || p.ctl == nil {
		return
	}
	ctl := p.ctl
	type result struct {
		state S
		err   error
	}
	async(p.k, func() result {
		st, err := callWithin(shellCallWait, ctl.read)
		return result{st, err}
	}, func(r result) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.unread = true
			p.fail(r.err)
			return
		}
		p.show(r.state)
	})
}

func (p *shellSwitch[S]) switched(on bool) {
	if p.syncing || p.ctl == nil {
		return
	}
	ctl := p.ctl
	p.problem.SetVisible(false)
	async(p.k, func() error {
		return doWithin(shellCallWait, func(ctx context.Context) error { return ctl.set(ctx, on) })
	}, func(err error) {
		if err != nil {
			p.fail(fmt.Errorf("%s: %w", locale.T().Get(p.spec.refused), err))
		}
		p.reload()
	})
}

func (p *shellSwitch[S]) fail(err error) {
	p.problem.SetText(err.Error())
	p.problem.SetVisible(true)
}

func (p *shellSwitch[S]) show(st S) {
	if p.unread {
		p.unread = false
		p.problem.SetVisible(false)
	}
	p.syncing = true
	p.toggle.SetEnabled(true)
	p.toggle.SetOn(p.spec.on(st))
	p.syncing = false
	p.state, p.known = st, true
	p.status.SetText(p.spec.words(p.k, st))
}

// configChanged says the state again, in words that may follow the
// config.
func (p *shellSwitch[S]) configChanged() {
	if p.known {
		p.status.SetText(p.spec.words(p.k, p.state))
	}
}

func (p *shellSwitch[S]) close() {
	p.closed = true
	if p.stop != nil {
		p.stop()
	}
	if p.ctl != nil {
		_ = p.ctl.close()
		p.ctl = nil
	}
}
