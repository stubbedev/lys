package backend

import (
	"fmt"
	"time"

	"github.com/stubbedev/lys/wm/internal/loop"
)

type VblankTimer struct {
	timer   *loop.Timer
	now     func() time.Duration
	armed   bool
	last    time.Duration
	next    time.Duration
	seq     uint64
	nextSeq uint64
}

func NewVblankTimer(l *loop.Loop, fired func() error) (*VblankTimer, error) {
	v := &VblankTimer{now: l.Now}
	timer, err := l.AddTimer(func() error {
		v.armed = false
		v.last, v.seq = v.next, v.nextSeq
		return fired()
	})
	if err != nil {
		return nil, fmt.Errorf("backend: add a vblank timer: %w", err)
	}
	v.timer = timer
	return v, nil
}

func (v *VblankTimer) Arm(interval time.Duration) error {
	if v.armed {
		return nil
	}
	now := v.now()
	at, ticks := NextVblank(v.last, now, interval)
	if err := v.timer.ArmFrame(at - now); err != nil {
		return fmt.Errorf("backend: arm the vblank timer for %v: %w", at, err)
	}
	v.next, v.nextSeq, v.armed = at, v.seq+ticks, true
	return nil
}

func (v *VblankTimer) Last() time.Duration {
	return v.last
}

func (v *VblankTimer) Seq() uint64 {
	return v.seq
}

func (v *VblankTimer) Remove() error {
	return v.timer.Remove()
}
