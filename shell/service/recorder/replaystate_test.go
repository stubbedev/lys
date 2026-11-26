package recorder_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/recorder"
	"github.com/stubbedev/lys/shell/service/recorder/recordertest"
)

func armedReplay(t *testing.T, engine *recordertest.Engine, s *recorder.State) *recordertest.Replay {
	t.Helper()
	waittest.Until(t, func() bool { return s.Snapshot().Replay == recorder.ReplayArmed }, "the replay armed")
	replays := engine.Replays()
	if len(replays) != 1 {
		t.Fatalf("%d replays started, want 1", len(replays))
	}
	return replays[0]
}

// Switching replay.enable on arms the replay on the configured output
// with the configured span; a keyframe updates the memory the snapshot
// reports; a save writes a new file in the output directory without
// stopping it, and says where; switching enable off stops the capture
// and closes the session.
func TestReplayArmsSavesAndDisarmsWithTheConfig(t *testing.T) {
	engine := &recordertest.Engine{}
	s, log := newState(t, engine, func(c *config.RecorderConfig) {
		c.OutputFormat = config.RecorderFormatMp4
		c.Replay.Seconds = 45
	})
	s.ConfigureReplay(false)
	if snap := s.Snapshot(); snap.Replay != recorder.ReplayOff || len(engine.Outputs()) != 0 {
		t.Fatalf("enable left off armed %+v (outputs %v)", snap, engine.Outputs())
	}
	s.ConfigureReplay(true)
	rp := armedReplay(t, engine, s)
	if got := engine.Outputs(); !slices.Equal(got, []string{config.ReplayOutputFocused}) || rp.Span != 45*time.Second {
		t.Errorf("opened %v with span %v, want the focused output and 45s", got, rp.Span)
	}
	rp.Keyframe(5 << 20)
	if got := s.Snapshot().ReplayBytes; got != 5<<20 {
		t.Errorf("snapshot reports %d bytes, want the ring's %d", got, 5<<20)
	}
	path, err := s.SaveReplay()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".mp4" || !strings.HasPrefix(filepath.Base(path), "lys-shell-replay-") {
		t.Errorf("saved to %s", path)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "replay mp4" {
		t.Errorf("the saved file holds %q (%v)", data, err)
	}
	if rp.Stops() != 0 || s.Snapshot().Replay != recorder.ReplayArmed {
		t.Errorf("a save stopped the replay: %d stops, %v", rp.Stops(), s.Snapshot().Replay)
	}
	toasts, notices := log.snapshot()
	if !slices.Contains(toasts, i18n.T("recorder-toast-replay-saved")) || !slices.Contains(log.savedSnapshot(), i18n.T("recorder-notification-replay-saved")+": "+path) || len(notices) != 0 {
		t.Errorf("toasts %q, saved %q, plain notices %q", toasts, log.savedSnapshot(), notices)
	}
	s.ConfigureReplay(true)
	if len(engine.Replays()) != 1 {
		t.Error("an unchanged enable armed a second replay")
	}
	s.ConfigureReplay(false)
	if snap := s.Snapshot(); snap.Replay != recorder.ReplayOff || snap.ReplayBytes != 0 {
		t.Errorf("after disabling = %+v", snap)
	}
	if rp.Stops() != 1 || engine.Sessions()[0].Closed() != 1 {
		t.Errorf("stops %d, session closes %d: the capture was left open", rp.Stops(), engine.Sessions()[0].Closed())
	}
}

// With no replay armed a save is the defined error and writes nothing.
func TestReplaySaveWhileOffFails(t *testing.T) {
	engine := &recordertest.Engine{}
	var dir string
	s, _ := newState(t, engine, func(c *config.RecorderConfig) { dir = c.OutputDirectory })
	if _, err := s.SaveReplay(); !errors.Is(err, recorder.ErrReplayOff) {
		t.Fatalf("save = %v, want %v", err, recorder.ErrReplayOff)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the output directory was touched: %v", err)
	}
}

// One save writes at a time: a second while the first is writing is
// the defined error and writes nothing; the snapshot says a save is
// writing until it returns, and a save after it writes again.
func TestReplaySaveWhileOneIsWritingFails(t *testing.T) {
	engine := &recordertest.Engine{}
	s, _ := newState(t, engine, nil)
	s.StartReplay()
	rp := armedReplay(t, engine, s)
	if s.Snapshot().ReplaySaving {
		t.Fatal("an armed replay with no save under way reports one writing")
	}
	release := rp.HoldSave()
	first := make(chan error, 1)
	go func() {
		_, err := s.SaveReplay()
		first <- err
	}()
	waittest.Until(t, func() bool { return s.Snapshot().ReplaySaving }, "the first save writing")
	second := make(chan error, 1)
	go func() {
		_, err := s.SaveReplay()
		second <- err
	}()
	secondErr := <-second
	release()
	if err := <-first; err != nil {
		t.Fatalf("the first save: %v", err)
	}
	if !errors.Is(secondErr, recorder.ErrReplaySaving) {
		t.Errorf("a save while one writes = %v, want %v", secondErr, recorder.ErrReplaySaving)
	}
	if s.Snapshot().ReplaySaving {
		t.Error("the snapshot still reports a save writing after it returned")
	}
	if got := len(rp.Saves()); got != 1 {
		t.Errorf("%d files written, want the first save's only", got)
	}
	if _, err := s.SaveReplay(); err != nil {
		t.Fatalf("a save after the first returned: %v", err)
	}
	if got := len(rp.Saves()); got != 2 {
		t.Errorf("%d files written, want a second once the first returned", got)
	}
}

// The CLI's start and stop work whatever the config says, and a
// recording runs beside an armed replay without touching it.
func TestReplayFromTheCLIBesideARecording(t *testing.T) {
	engine := &recordertest.Engine{WriteOutput: true}
	s, _ := newState(t, engine, func(c *config.RecorderConfig) { c.Replay.Output = "DP-2" })
	s.StartReplay()
	s.StartReplay()
	rp := armedReplay(t, engine, s)
	if got := engine.Outputs(); !slices.Equal(got, []string{"DP-2"}) {
		t.Errorf("opened %v, want DP-2 once", got)
	}
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Stop()
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
	if rp.Stops() != 0 || s.Snapshot().Replay != recorder.ReplayArmed {
		t.Errorf("the recording stopped the replay: %d stops", rp.Stops())
	}
	s.StopReplay()
	s.StopReplay()
	if rp.Stops() != 1 {
		t.Errorf("%d stops, want 1", rp.Stops())
	}
}

// A replay that cannot open its output, or whose capture dies, settles
// off and says why; a failed save leaves it armed and reports it.
func TestReplayFailuresAreReported(t *testing.T) {
	logged := logtest.Tee(t)
	engine := &recordertest.Engine{OpenErr: errors.New("recorder: the output is gone: DP-9")}
	s, log := newState(t, engine, nil)
	s.StartReplay()
	waittest.Until(t, func() bool { toasts, _ := log.snapshot(); return len(toasts) > 0 }, "the failure report")
	if snap := s.Snapshot(); snap.Replay != recorder.ReplayOff {
		t.Errorf("after a failed open = %+v", snap)
	}
	wantFailure(t, "a failed open", log, logged, i18n.T("recorder-toast-replay-failed"), "DP-9")

	engine = &recordertest.Engine{}
	s, log = newState(t, engine, nil)
	s.StartReplay()
	rp := armedReplay(t, engine, s)
	rp.FailSaves(errors.New("disk full"))
	if _, err := s.SaveReplay(); err == nil {
		t.Error("a failing save succeeded")
	}
	if s.Snapshot().Replay != recorder.ReplayArmed {
		t.Error("a failed save disarmed the replay")
	}
	rp.Fail(errors.New("recorder: the output is gone: DP-1"))
	waittest.Until(t, func() bool { return s.Snapshot().Replay == recorder.ReplayOff }, "the replay to be off")
	if rp.Stops() != 1 || engine.Sessions()[0].Closed() != 1 {
		t.Errorf("stops %d, closes %d after the capture died", rp.Stops(), engine.Sessions()[0].Closed())
	}
	wantFailure(t, "a failed save", log, logged, i18n.T("recorder-toast-replay-failed"), "disk full")
	wantFailure(t, "a capture that died", log, logged, i18n.T("recorder-toast-replay-failed"), "DP-1")
	if toasts, _ := log.snapshot(); slices.ContainsFunc(toasts, func(s string) bool { return strings.Contains(s, "disk full") || strings.Contains(s, "DP-1") }) {
		t.Errorf("toasts %q carry an error", toasts)
	}
}

func TestReplayPath(t *testing.T) {
	now := time.Date(2026, 11, 7, 9, 5, 3, 120_000_000, time.Local)
	got := recorder.ReplayPath("/v", config.RecorderFormatMkv, now)
	if got != "/v/lys-shell-replay-20261107-090503.120.mkv" {
		t.Errorf("ReplayPath = %s", got)
	}
	if other := recorder.ReplayPath("/v", config.RecorderFormatMkv, now.Add(time.Millisecond)); other == got {
		t.Error("two saves a millisecond apart share a name")
	}
}

func TestSessionCloseFailureOfTheReplayIsReported(t *testing.T) {
	closeErr := errors.New("recorder: close the compositor connection capturing DP-1: broken pipe")
	startErr := errors.New("recorder: a 64x48 H.264 encoder: out of memory")
	engine := &recordertest.Engine{StartErr: startErr, CloseErr: closeErr}
	logged := logtest.Tee(t)
	s, log := newState(t, engine, nil)
	s.StartReplay()
	waittest.Until(t, func() bool { toasts, _ := log.snapshot(); return len(toasts) > 0 }, "the failure report")
	wantFailure(t, "a failed start", log, logged, i18n.T("recorder-toast-replay-failed"), startErr.Error())
	wantFailure(t, "a failed start", log, logged, i18n.T("recorder-toast-replay-failed"), closeErr.Error())

	engine = &recordertest.Engine{CloseErr: closeErr}
	s, _ = newState(t, engine, nil)
	s.StartReplay()
	armedReplay(t, engine, s)
	s.StopReplay()
	if got := logged.String(); !strings.Contains(got, "stop the instant replay: "+closeErr.Error()) {
		t.Errorf("log = %q, want the close error of the disarmed replay", got)
	}
	if got := engine.Sessions()[0].Closed(); got != 1 {
		t.Errorf("session closes %d, want 1", got)
	}
}

// holdASave arms a replay and starts a save that stays writing until
// release runs; saved delivers what the save returned.
func holdASave(t *testing.T) (s *recorder.State, engine *recordertest.Engine, rp *recordertest.Replay, release func(), saved <-chan error) {
	t.Helper()
	engine = &recordertest.Engine{}
	s, _ = newState(t, engine, func(c *config.RecorderConfig) { c.OutputFormat = config.RecorderFormatMkv })
	s.StartReplay()
	rp = armedReplay(t, engine, s)
	release = rp.HoldSave()
	t.Cleanup(release)
	done := make(chan error, 1)
	go func() {
		path, err := s.SaveReplay()
		if err == nil {
			data, readErr := os.ReadFile(path)
			if readErr != nil || string(data) != "replay mkv" {
				err = errors.Join(readErr, errors.New("the saved file holds "+string(data)))
			}
		}
		done <- err
	}()
	waittest.Until(t, func() bool { return s.Snapshot().ReplaySaving }, "the save writing")
	return s, engine, rp, release, done
}

// A stop while a save writes the replay turns it off at once, but
// stops the capture only once the save returned, so the file is whole;
// the stop returns after the capture stopped.
func TestReplayStopWaitsForTheSaveWritingIt(t *testing.T) {
	s, engine, rp, release, saved := holdASave(t)
	stopped := make(chan struct{})
	go func() {
		s.StopReplay()
		close(stopped)
	}()
	waittest.Until(t, func() bool { return s.Snapshot().Replay == recorder.ReplayOff }, "the replay off")
	if rp.Stops() != 0 || engine.Sessions()[0].Closed() != 0 {
		t.Fatalf("stops %d, session closes %d while the save still writes", rp.Stops(), engine.Sessions()[0].Closed())
	}
	release()
	if err := <-saved; err != nil {
		t.Fatalf("the save under the stop: %v", err)
	}
	<-stopped
	if rp.Stops() != 1 || rp.StopsMidSave() != 0 || engine.Sessions()[0].Closed() != 1 {
		t.Errorf("stops %d (%d mid-save), session closes %d, want one stop after the save", rp.Stops(), rp.StopsMidSave(), engine.Sessions()[0].Closed())
	}
}

// A capture dying while a save writes the replay is torn down once the
// save returned, and still reported.
func TestReplayCaptureDeathWaitsForTheSaveWritingIt(t *testing.T) {
	s, engine, rp, release, saved := holdASave(t)
	torndown := make(chan struct{})
	go func() {
		rp.Fail(errors.New("recorder: the output is gone: DP-1"))
		close(torndown)
	}()
	waittest.Until(t, func() bool { return s.Snapshot().Replay == recorder.ReplayOff }, "the replay off")
	if rp.Stops() != 0 || engine.Sessions()[0].Closed() != 0 {
		t.Fatalf("stops %d, session closes %d while the save still writes", rp.Stops(), engine.Sessions()[0].Closed())
	}
	release()
	if err := <-saved; err != nil {
		t.Fatalf("the save under the capture death: %v", err)
	}
	<-torndown
	if rp.Stops() != 1 || rp.StopsMidSave() != 0 || engine.Sessions()[0].Closed() != 1 {
		t.Errorf("stops %d (%d mid-save), session closes %d, want one stop after the save", rp.Stops(), rp.StopsMidSave(), engine.Sessions()[0].Closed())
	}
}

// A stop of a replay no save is writing does not wait on a save still
// writing an earlier replay.
func TestReplayStopDoesNotWaitForASaveOfAnotherReplay(t *testing.T) {
	s, engine, first, release, saved := holdASave(t)
	firstStopped := make(chan struct{})
	go func() {
		s.StopReplay()
		close(firstStopped)
	}()
	waittest.Until(t, func() bool { return s.Snapshot().Replay == recorder.ReplayOff }, "the first replay off")
	s.StartReplay()
	waittest.Until(t, func() bool {
		return len(engine.Replays()) == 2 && s.Snapshot().Replay == recorder.ReplayArmed
	}, "the second replay armed")
	second := engine.Replays()[1]
	s.StopReplay()
	if second.Stops() != 1 || engine.Sessions()[1].Closed() != 1 {
		t.Errorf("the second replay: stops %d, session closes %d, want it stopped without waiting", second.Stops(), engine.Sessions()[1].Closed())
	}
	if first.Stops() != 0 {
		t.Errorf("the first replay stopped %d times while its save still writes", first.Stops())
	}
	release()
	if err := <-saved; err != nil {
		t.Fatalf("the first replay's save: %v", err)
	}
	<-firstStopped
	if first.Stops() != 1 || first.StopsMidSave() != 0 {
		t.Errorf("the first replay: stops %d (%d mid-save)", first.Stops(), first.StopsMidSave())
	}
}
