package recorder

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/stubbedev/lys/desktop/i18n"
)

// ReplayStatus is the instant replay's lifecycle.
type ReplayStatus int

// Replay statuses.
const (
	ReplayOff ReplayStatus = iota
	ReplayStarting
	ReplayArmed
)

// String names the status.
func (s ReplayStatus) String() string {
	switch s {
	case ReplayStarting:
		return "starting"
	case ReplayArmed:
		return "armed"
	}
	return "off"
}

// ErrReplayOff is a save with no instant replay armed.
var ErrReplayOff = errors.New("recorder: the instant replay is not armed")

// ErrReplaySaving is a save while another is still writing.
var ErrReplaySaving = errors.New("recorder: an instant replay save is already writing")

const replayIcon = "ld-history-symbolic"

// ConfigureReplay applies modules.recorder.replay.enable: switching it
// on arms the replay, off disarms it. An unchanged value leaves a replay
// started or stopped from the CLI as it is.
func (s *State) ConfigureReplay(enable bool) {
	s.mu.Lock()
	changed := s.replayWanted != enable
	s.replayWanted = enable
	s.mu.Unlock()
	switch {
	case !changed:
	case enable:
		s.StartReplay()
	default:
		s.StopReplay()
	}
}

// StartReplay arms the instant replay on the configured output. The
// Off→Starting claim is synchronous; opening the output and starting
// the capture run off the caller.
func (s *State) StartReplay() {
	s.mu.Lock()
	if s.replayStatus != ReplayOff {
		s.mu.Unlock()
		return
	}
	s.replayStatus = ReplayStarting
	s.replayGen++
	gen := s.replayGen
	s.mu.Unlock()
	s.notify()
	go s.runReplay(gen)
}

func (s *State) runReplay(gen uint64) {
	cfg := s.config()
	opts := BuildOptions(cfg, s.now())
	session, err := s.engine.OpenOutput(cfg.Replay.Output, opts.ShowCursor)
	if err != nil {
		s.replayFailed(gen, err)
		return
	}
	rep, err := session.StartReplay(opts, cfg.Replay.Seconds.Span(), func(err error) { s.replayEnded(gen, err) }, s.notify)
	if err != nil {
		s.replayFailed(gen, errors.Join(err, session.Close()))
		return
	}
	s.mu.Lock()
	if s.replayStatus != ReplayStarting || s.replayGen != gen {
		s.mu.Unlock()
		stopReplay(rep, session)
		return
	}
	s.replayStatus = ReplayArmed
	s.replay, s.replaySession = rep, session
	s.mu.Unlock()
	s.notify()
	s.toast(i18n.T("recorder-toast-replay-armed"), replayIcon, startToastMS)
}

// StopReplay disarms the instant replay, freeing its ring once a save
// still writing it returns; one still starting is torn down as it comes
// up.
func (s *State) StopReplay() {
	s.mu.Lock()
	if s.replayStatus == ReplayOff {
		s.mu.Unlock()
		return
	}
	s.replayGen++
	teardown := s.takeReplayLocked()
	s.mu.Unlock()
	s.notify()
	teardown.run()
}

// replaySave is a save writing rep; done closes once it returns.
type replaySave struct {
	rep  Replay
	done chan struct{}
}

// replayTeardown is what an armed replay ran on, and the save writing
// it when one was.
type replayTeardown struct {
	rep     Replay
	session Session
	saving  <-chan struct{}
}

// takeReplayLocked turns the replay off and hands back its teardown,
// with no replay while it was only starting. The caller holds mu.
func (s *State) takeReplayLocked() replayTeardown {
	teardown := replayTeardown{rep: s.replay, session: s.replaySession}
	if s.replaySave != nil && s.replaySave.rep == teardown.rep {
		teardown.saving = s.replaySave.done
	}
	s.replayStatus = ReplayOff
	s.replay, s.replaySession = nil, nil
	return teardown
}

// run waits for the save writing the replay to return, then stops the
// capture and closes its session.
func (t replayTeardown) run() {
	if t.rep == nil {
		return
	}
	if t.saving != nil {
		<-t.saving
	}
	stopReplay(t.rep, t.session)
}

func stopReplay(rep Replay, session Session) {
	if err := endCapture(rep, session); err != nil {
		log.Printf("recorder: stop the instant replay: %v", err)
	}
}

// SaveReplay writes the replay's last seconds to a new file in the
// output directory, without stopping it, and returns the file. One save
// writes at a time: another while it does fails with ErrReplaySaving.
func (s *State) SaveReplay() (string, error) {
	s.mu.Lock()
	rep := s.replay
	switch {
	case rep == nil:
		s.mu.Unlock()
		return "", ErrReplayOff
	case s.replaySave != nil:
		s.mu.Unlock()
		return "", ErrReplaySaving
	}
	saving := &replaySave{rep: rep, done: make(chan struct{})}
	s.replaySave = saving
	s.mu.Unlock()
	s.notify()
	defer s.replaySaved(saving)
	cfg := s.config()
	path := ReplayPath(cfg.OutputDirectory, cfg.OutputFormat, s.now())
	if err := os.MkdirAll(filepath.Dir(path), outputDirPerms); err != nil {
		return "", fmt.Errorf("recorder: the output directory: %w", err)
	}
	if err := rep.Save(path, cfg.OutputFormat); err != nil {
		log.Printf("recorder: save the instant replay to %s: %v", path, err)
		s.showError(i18n.T("recorder-toast-replay-failed"))
		return "", err
	}
	s.toast(i18n.T("recorder-toast-replay-saved"), replayIcon, stopToastMS)
	if s.hooks.Saved != nil {
		s.hooks.Saved(i18n.T("recorder-notification-replay-saved"), path, replayIcon)
	}
	return path, nil
}

// replaySaved ends the save SaveReplay claimed, releasing a teardown
// waiting on it.
func (s *State) replaySaved(saving *replaySave) {
	s.mu.Lock()
	s.replaySave = nil
	s.mu.Unlock()
	close(saving.done)
	s.notify()
}

// replayFailed settles a start that failed to Off, unless a stop or a
// newer start came first, and reports it.
func (s *State) replayFailed(gen uint64, err error) {
	s.mu.Lock()
	if s.replayGen != gen || s.replayStatus != ReplayStarting {
		s.mu.Unlock()
		return
	}
	s.replayStatus = ReplayOff
	s.mu.Unlock()
	log.Printf("recorder: failed to arm the instant replay: %v", err)
	s.notify()
	s.showError(i18n.T("recorder-toast-replay-failed"))
}

// replayEnded tears down an armed replay whose capture died on its own.
func (s *State) replayEnded(gen uint64, err error) {
	s.mu.Lock()
	if s.replayGen != gen || s.replayStatus != ReplayArmed {
		s.mu.Unlock()
		return
	}
	s.replayGen++
	teardown := s.takeReplayLocked()
	s.mu.Unlock()
	log.Printf("recorder: the instant replay ended unexpectedly: %v", err)
	s.notify()
	teardown.run()
	s.showError(i18n.T("recorder-toast-replay-failed"))
}
