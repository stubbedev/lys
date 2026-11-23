package settings

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/stubbedev/lys/ui/widget"

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
// whether it is on and the line saying it, with a time it names as the
// clock shows it or, with the error why it cannot, as a 24-hour time,
// and the message keys of its row, its waiting, its absence and a
// refused switch.
type switchSpec[S any] struct {
	open                        func(k *kit) (switchControl[S], error)
	on                          func(S) bool
	words                       func(k *kit, s S) (string, error)
	label, loading, unavailable string
	refused                     string
}

// shellSwitch is a row switching a lys-shell state as it stands, its
// description following every change lys-shell announces and its
// status saying, under it, that lys-shell is being asked, or why a
// switch or a read failed. It is a row of the card it is appended to.
type shellSwitch[S any] struct {
	*settingRow
	k       *kit
	spec    switchSpec[S]
	ctl     switchControl[S]
	stop    context.CancelFunc
	toggle  *widget.Switch
	status  *inlineState
	state   S
	known   bool
	syncing bool
	unread  bool
	// unclocked is set while the status says the clock's format cannot
	// be read.
	unclocked bool
	closed    bool
}

func newShellSwitch[S any](k *kit, spec switchSpec[S]) *shellSwitch[S] {
	t := locale.T()
	p := &shellSwitch[S]{settingRow: k.row(t.Get(spec.label), ""), k: k, spec: spec, toggle: widget.NewSwitch(false), status: k.inlineState()}
	p.status.Loading(t.Get(spec.loading))
	p.AddDetail(p.status)
	p.toggle.SetAccessibleName(t.Get(spec.label))
	p.toggle.SetEnabled(false)
	p.toggle.OnChanged = p.switched
	p.withControl(p.toggle)
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
			log.Printf("settings: %s: %v", spec.label, o.err)
			p.status.Unavailable("", t.Get(spec.unavailable), o.err)
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
		p.fail(p.spec.unavailable, err)
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
			p.fail(p.spec.unavailable, r.err)
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
	p.status.Clear()
	async(p.k, func() error {
		return doWithin(shellCallWait, func(ctx context.Context) error { return ctl.set(ctx, on) })
	}, func(err error) {
		if err != nil {
			p.fail(p.spec.refused, err)
		}
		p.reload()
	})
}

// fail says the message key's sentence over err.
func (p *shellSwitch[S]) fail(key string, err error) {
	p.status.Error(locale.T().Get(key), err, nil)
}

func (p *shellSwitch[S]) show(st S) {
	if p.unread || p.status.kind == stateLoading {
		p.unread = false
		p.status.Clear()
	}
	p.syncing = true
	p.toggle.SetEnabled(true)
	p.toggle.SetOn(p.spec.on(st))
	p.syncing = false
	p.state, p.known = st, true
	p.describe()
}

// describe says the state in the row's description, and on the status
// when its time cannot be shown as the clock shows times.
func (p *shellSwitch[S]) describe() {
	words, err := p.spec.words(p.k, p.state)
	p.SetDescription(words)
	switch {
	case err != nil:
		p.unclocked = true
		p.fail("settings-clock-format-unread", err)
	case p.unclocked:
		p.unclocked = false
		p.status.Clear()
	}
}

// configChanged says the state again, in words that may follow the
// config.
func (p *shellSwitch[S]) configChanged() {
	if p.known {
		p.describe()
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
