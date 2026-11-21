// Package idle is the shell's built-in idle policy: after the session has
// been idle for a stage's timeout it turns the keyboard backlights off,
// dims the backlights, locks, turns the outputs off and suspends, each undone (where it can be) when the user is
// back, with timeouts per power source; and it locks before the system
// sleeps. Idle is the compositor's ext-idle-notify-v1, so idle inhibitors
// (a playing video, the caffeine toggle, a portal or ScreenSaver hold)
// hold every stage, and waiting costs no timer. With attention on, the
// dim first asks lys-power whether a face is turned to the screen: a
// face postpones it, and the stages after it follow it on timers.
package idle

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/power/attention"
)

// Watcher tells when the session has been idle for a while: idle runs once
// it has been idle for after, resume when the user is back. Both run on
// the watcher's own goroutine, and idle inhibitors hold the idle off.
type Watcher interface {
	Watch(after time.Duration, idle, resume func()) (stop func(), err error)
}

// Actions carry the stages out, and Look checks for a face before the
// dim. KeyboardOff, Dim and ScreenOff return what undoes them at resume.
type Actions interface {
	KeyboardOff() (restore func() error, err error)
	Dim(idlepolicy.DimPercent) (restore func() error, err error)
	Lock() error
	ScreenOff() (restore func() error, err error)
	Sleep(logind.PowerAction) error
	Look(ctx context.Context) (idlepolicy.Presence, error)
}

// lookTimeout bounds one check for a face, a capture of under a second
// behind a D-Bus activation.
const lookTimeout = 15 * time.Second

// Status is what the service has armed and which stages have run since the
// user was last active.
type Status struct {
	Enabled bool
	Source  idlepolicy.PowerSource
	Arms    []idlepolicy.Arm
	Reached []idlepolicy.Stage
}

// Next is the first armed stage that has not run yet; false when every
// armed one has, or none is.
func (s Status) Next() (idlepolicy.Arm, bool) {
	for _, a := range s.Arms {
		if !slices.Contains(s.Reached, a.Stage) {
			return a, true
		}
	}
	return idlepolicy.Arm{}, false
}

// Service arms one idle watch per stage of the policy on the current power
// source, and re-arms them when either changes.
type Service struct {
	watcher   Watcher
	actions   Actions
	afterFunc func(time.Duration, func()) (stop func())

	mu        sync.Mutex
	policy    idlepolicy.Policy
	enabled   bool
	source    idlepolicy.PowerSource
	stops     []func()
	restores  [len(idlepolicy.Stages)]func() error
	reached   []idlepolicy.Stage
	gen       uint64
	changes   feed.Tick
	round     uint64
	postponed int
	timers    []func()
	looking   context.CancelFunc
	noCamera  bool
}

// New is a service that has armed nothing until Configure.
func New(w Watcher, a Actions) *Service {
	return &Service{watcher: w, actions: a, afterFunc: realAfterFunc}
}

func realAfterFunc(d time.Duration, f func()) func() {
	t := time.AfterFunc(d, f)
	return func() { t.Stop() }
}

// Configure arms the policy's stages on source; disabled arms none. Every
// stage that has run is undone first, so a new policy starts from an
// active session. Once lys-power has answered that there is no camera,
// the policy runs without attention.
func (s *Service) Configure(p idlepolicy.Policy, enabled bool, source idlepolicy.PowerSource) error {
	s.mu.Lock()
	if s.noCamera {
		p.Attention = idlepolicy.AttentionOff
	}
	s.policy, s.enabled, s.source = p, enabled, source
	err := s.rearmLocked()
	s.mu.Unlock()
	feed.Notify(&s.changes)
	return err
}

// SetPowerSource re-arms the stages with source's timeouts.
func (s *Service) SetPowerSource(source idlepolicy.PowerSource) error {
	s.mu.Lock()
	if s.source == source {
		s.mu.Unlock()
		return nil
	}
	s.source = source
	err := s.rearmLocked()
	s.mu.Unlock()
	feed.Notify(&s.changes)
	return err
}

// Policy is the policy the service runs.
func (s *Service) Policy() idlepolicy.Policy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

// Status reads what is armed and reached.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{Enabled: s.enabled, Source: s.source, Reached: slices.Clone(s.reached)}
	if s.enabled {
		st.Arms = s.policy.Arms(s.source)
	}
	return st
}

// Changes ticks after the service re-arms or a stage runs or is undone.
func (s *Service) Changes() (<-chan struct{}, func()) { return s.changes.Subscribe() }

// Close stops every watch and undoes what the stages did.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	return s.disarmLocked()
}

func (s *Service) disarmLocked() error {
	for _, stop := range s.stops {
		stop()
	}
	s.stops = s.stops[:0]
	s.forgetDimLocked()
	var err error
	for _, stage := range idlepolicy.Stages {
		err = errors.Join(err, s.restoreLocked(stage))
	}
	s.reached = s.reached[:0]
	return err
}

func (s *Service) rearmLocked() error {
	s.gen++
	err := s.disarmLocked()
	if !s.enabled {
		return err
	}
	gen := s.gen
	for _, arm := range s.policy.Watched(s.source) {
		stage := arm.Stage
		stop, werr := s.watcher.Watch(arm.After, func() { s.idled(gen, stage) }, func() { s.resumed(gen, stage) })
		if werr != nil {
			return errors.Join(err, fmt.Errorf("idle: watch %s after %s: %w", stage, arm.After, werr))
		}
		s.stops = append(s.stops, stop)
	}
	return err
}

func (s *Service) idled(gen uint64, stage idlepolicy.Stage) {
	s.mu.Lock()
	if gen != s.gen || slices.Contains(s.reached, stage) {
		s.mu.Unlock()
		return
	}
	if stage == idlepolicy.Dim && s.policy.Attending(s.source) {
		switch {
		case s.noCamera:
			s.lookedLocked(gen, s.round, idlepolicy.Unseen)
			return
		case s.looking == nil:
			s.lookLocked(gen)
		}
		s.mu.Unlock()
		return
	}
	s.reachLocked(stage)
}

func (s *Service) followed(gen, round uint64, stage idlepolicy.Stage) {
	s.mu.Lock()
	if gen != s.gen || round != s.round || slices.Contains(s.reached, stage) {
		s.mu.Unlock()
		return
	}
	s.reachLocked(stage)
}

func (s *Service) reachLocked(stage idlepolicy.Stage) {
	s.reached = append(s.reached, stage)
	err := s.runLocked(stage)
	s.mu.Unlock()
	if err != nil {
		log.Printf("idle: %s: %v", stage, err)
	}
	feed.Notify(&s.changes)
}

func (s *Service) lookLocked(gen uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), lookTimeout)
	s.looking = cancel
	round := s.round
	go func() {
		defer cancel()
		seen, err := s.actions.Look(ctx)
		noCamera := errors.Is(err, attention.ErrNoCamera)
		switch {
		case noCamera:
			log.Printf("idle: %v; the dim stops checking for a face", err)
			seen = idlepolicy.Unseen
		case err != nil:
			log.Printf("idle: the check for a face before the dim failed, dimming as without it: %v", err)
			seen = idlepolicy.Unseen
		}
		s.mu.Lock()
		s.noCamera = s.noCamera || noCamera
		s.lookedLocked(gen, round, seen)
	}()
}

func (s *Service) lookedLocked(gen, round uint64, seen idlepolicy.Presence) {
	if gen != s.gen || round != s.round {
		s.mu.Unlock()
		return
	}
	s.looking = nil
	step := s.policy.AtDim(s.source, seen, s.postponed)
	if step.Postpone > 0 {
		s.postponed++
		s.timers = append(s.timers, s.afterFunc(step.Postpone, func() { s.lookAgain(gen, round) }))
		s.mu.Unlock()
		feed.Notify(&s.changes)
		return
	}
	for _, f := range s.policy.Followers(s.source, step.Hurried) {
		stage := f.Stage
		s.timers = append(s.timers, s.afterFunc(f.After, func() { s.followed(gen, round, stage) }))
	}
	s.reachLocked(idlepolicy.Dim)
}

func (s *Service) lookAgain(gen, round uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if gen != s.gen || round != s.round || s.looking != nil {
		return
	}
	s.lookLocked(gen)
}

func (s *Service) forgetDimLocked() {
	for _, stop := range s.timers {
		stop()
	}
	s.timers = s.timers[:0]
	if s.looking != nil {
		s.looking()
		s.looking = nil
	}
	s.round++
	s.postponed = 0
}

func (s *Service) runLocked(stage idlepolicy.Stage) error {
	var (
		restore func() error
		err     error
	)
	switch stage {
	case idlepolicy.KeyboardBacklight:
		restore, err = s.actions.KeyboardOff()
	case idlepolicy.Dim:
		restore, err = s.actions.Dim(s.policy.DimPercent)
	case idlepolicy.Lock:
		err = s.actions.Lock()
	case idlepolicy.ScreenOff:
		restore, err = s.actions.ScreenOff()
	case idlepolicy.Suspend:
		err = s.actions.Sleep(s.policy.Sleep)
	}
	s.restores[stage] = restore
	return err
}

func (s *Service) resumed(gen uint64, stage idlepolicy.Stage) {
	s.mu.Lock()
	if gen != s.gen {
		s.mu.Unlock()
		return
	}
	undone := []idlepolicy.Stage{stage}
	if stage == idlepolicy.Dim && s.policy.Attending(s.source) {
		s.forgetDimLocked()
		for _, f := range s.policy.Followers(s.source, false) {
			undone = append(undone, f.Stage)
		}
	}
	s.reached = slices.DeleteFunc(s.reached, func(r idlepolicy.Stage) bool { return slices.Contains(undone, r) })
	var err error
	for _, u := range undone {
		err = errors.Join(err, s.restoreLocked(u))
	}
	if s.noCamera && s.policy.Attention != idlepolicy.AttentionOff {
		s.policy.Attention = idlepolicy.AttentionOff
		err = errors.Join(err, s.rearmLocked())
	}
	s.mu.Unlock()
	if err != nil {
		log.Printf("idle: undo %s: %v", stage, err)
	}
	feed.Notify(&s.changes)
}

func (s *Service) restoreLocked(stage idlepolicy.Stage) error {
	restore := s.restores[stage]
	s.restores[stage] = nil
	if restore == nil {
		return nil
	}
	if err := restore(); err != nil {
		return fmt.Errorf("idle: undo %s: %w", stage, err)
	}
	return nil
}
