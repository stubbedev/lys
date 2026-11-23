package recorder_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/recorder"
	"github.com/stubbedev/lys/shell/service/recorder/recordertest"
)

// hookLog records the toasts and notifications.
type hookLog struct {
	mu      sync.Mutex
	toasts  []string
	notices []string
	apps    []string
}

func (h *hookLog) hooks() recorder.Hooks {
	return recorder.Hooks{
		Toast: func(label, _ string, _ uint32) {
			h.mu.Lock()
			h.toasts = append(h.toasts, label)
			h.mu.Unlock()
		},
		Notify: func(app, summary, body, _ string) {
			h.mu.Lock()
			h.apps = append(h.apps, app)
			h.notices = append(h.notices, summary+": "+body)
			h.mu.Unlock()
		},
	}
}

// wantFailure requires a failure notice and toast saying summary, in
// the shell's words, with errText, the error, in the log alone.
func wantFailure(t *testing.T, what string, h *hookLog, logged *logtest.Log, summary, errText string) {
	t.Helper()
	toasts, notices := h.snapshot()
	if !slices.Contains(toasts, summary) || !slices.Contains(notices, summary+": ") {
		t.Errorf("%s: toasts %q, notices %q, want %q alone", what, toasts, notices, summary)
	}
	if !strings.Contains(logged.String(), errText) {
		t.Errorf("%s: log %q, want %q", what, logged.String(), errText)
	}
}

func (h *hookLog) snapshot() ([]string, []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.toasts), slices.Clone(h.notices)
}

func newState(t *testing.T, engine *recordertest.Engine, edit func(*config.RecorderConfig)) (*recorder.State, *hookLog) {
	t.Helper()
	cfg := config.Defaults().Recorder
	cfg.OutputDirectory = filepath.Join(t.TempDir(), "videos")
	cfg.StartDelayMs = 0
	if edit != nil {
		edit(&cfg)
	}
	log := &hookLog{}
	return recorder.NewState(engine, func() config.RecorderConfig { return cfg }, log.hooks(), time.Now), log
}

func TestRecordPauseStopSaves(t *testing.T) {
	// The gate holds the start at the portal session, so the claim
	// Start makes is what the snapshot shows, however fast the rest runs.
	gate := make(chan struct{})
	engine := &recordertest.Engine{WriteOutput: true, Gate: gate}
	s, log := newState(t, engine, func(c *config.RecorderConfig) {
		c.OutputFormat = config.RecorderFormatMp4
		c.Microphone, c.MicrophoneDevice, c.SystemAudio = true, "mic0", true
		c.WebcamEnabled, c.WebcamX, c.WebcamY, c.WebcamSize = true, 10, 90, 25
	})
	s.Start()
	if snap := s.Snapshot(); snap.Status != recorder.StatusStarting || !snap.Preparing || snap.Active {
		t.Fatalf("after Start = %+v, want a synchronous Starting claim", snap)
	}
	close(gate)
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	snap := s.Snapshot()
	if snap.Preparing || snap.Status != recorder.StatusRecording || filepath.Ext(snap.OutputPath) != ".mp4" {
		t.Fatalf("recording snapshot = %+v", snap)
	}
	rec := engine.Recordings()[0]
	opts := rec.Opts
	if opts.Format != config.RecorderFormatMp4 || !opts.Microphone || opts.MicrophoneDevice != "mic0" || !opts.SystemAudio ||
		opts.Webcam == nil || opts.Webcam.X != 10 || opts.Webcam.Y != 90 || opts.Webcam.Size != 25 {
		t.Errorf("options = %+v (webcam %+v)", opts, opts.Webcam)
	}
	if _, err := os.Stat(filepath.Dir(opts.OutputPath)); err != nil {
		t.Errorf("output dir not created: %v", err)
	}

	s.SetPaused(true)
	s.SetPaused(true) // no change: not forwarded
	if got := rec.Paused(); !slices.Equal(got, []bool{true}) || !s.Snapshot().Paused {
		t.Errorf("pause calls = %v", got)
	}
	s.SetPaused(false)

	release := rec.HoldStop()
	s.Stop()
	if snap := s.Snapshot(); snap.Active || snap.Status != recorder.StatusStopping {
		t.Errorf("right after Stop = %+v, want inactive and stopping", snap)
	}
	release()
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
	if rec.Stops() != 1 || engine.Sessions()[0].Closed() != 1 {
		t.Errorf("stops %d, session closes %d", rec.Stops(), engine.Sessions()[0].Closed())
	}
	waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, "the saved notice")
	toasts, notices := log.snapshot()
	if !slices.Contains(toasts, i18n.T("recorder-toast-starting")) || !slices.Contains(toasts, i18n.T("recorder-toast-stopped")) {
		t.Errorf("toasts = %q", toasts)
	}
	if notices[0] != i18n.T("recorder-notification-saved")+": "+opts.OutputPath {
		t.Errorf("notice = %q", notices[0])
	}
}

// A stop that leaves no file reports a failure, not "saved".
func TestStopWithoutOutputReportsFailure(t *testing.T) {
	logged := logtest.Tee(t)
	engine := &recordertest.Engine{}
	s, log := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Stop()
	waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, "the failure notice")
	wantFailure(t, "a stop without output", log, logged, i18n.T("recorder-toast-failed"), "produced no output file")
}

func TestStartClaimIsSynchronousAndStopCancelsTheStart(t *testing.T) {
	gate := make(chan struct{})
	engine := &recordertest.Engine{Gate: gate}
	s, _ := newState(t, engine, nil)
	s.Start()
	s.Start() // a rapid second start is a no-op
	s.Stop()  // cancels the start held at the picker
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
	close(gate)
	// The cancelled start reset to idle as its last step, so nothing
	// can record for it now.
	if len(engine.Recordings()) != 0 {
		t.Error("a cancelled start still recorded")
	}
	// Idle again: a new start proceeds.
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Stop()
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
}

// A start that fails at the picker or at the encoder settles to Idle
// with the reason as the module's error state, logged and notified; the
// next start clears it.
func TestFailedStartsResetWithTheirReason(t *testing.T) {
	logged := logtest.Tee(t)
	noEncoder := fmt.Errorf("recorder: %w: llvmpipe encodes no H.264", recorder.ErrNoVideoEncoder)
	for name, engine := range map[string]*recordertest.Engine{
		"picker":  {OpenErr: recorder.ErrPickerCancelled},
		"encoder": {StartErr: noEncoder},
	} {
		s, log := newState(t, engine, nil)
		s.Start()
		waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, name+" failure")
		snap := s.Snapshot()
		if snap.Status != recorder.StatusIdle || snap.Preparing || snap.Active {
			t.Errorf("%s: snapshot = %+v", name, snap)
		}
		want := recorder.ErrPickerCancelled.Error()
		if name == "encoder" {
			want = noEncoder.Error()
			if engine.Sessions()[0].Closed() != 1 {
				t.Error("an encoder failure leaked the session")
			}
		}
		if snap.Error != i18n.T("recorder-toast-failed") || strings.Contains(snap.Error, want) {
			t.Errorf("%s: error state %q, want the summary without %q", name, snap.Error, want)
		}
		if !strings.Contains(logged.String(), want) {
			t.Errorf("%s: log %q, want %q", name, logged.String(), want)
		}
		if _, n := log.snapshot(); n[0] != i18n.T("recorder-toast-failed")+": " {
			t.Errorf("%s: notice %q, want the summary without %q", name, n[0], want)
		}
		log.mu.Lock()
		app := log.apps[0]
		log.mu.Unlock()
		if want := i18n.T("notification-source-recorder"); app != want || app == "lys-shell" {
			t.Errorf("%s: notified as %q, want %q", name, app, want)
		}
		engine.OpenErr, engine.StartErr = nil, nil
		s.Start()
		if got := s.Snapshot().Error; got != "" {
			t.Errorf("%s: a new start kept the error %q", name, got)
		}
		waittest.Until(t, func() bool { return s.Snapshot().Active }, name+" recording")
		s.Stop()
		waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, name+" idle")
	}
}

// A recording whose file cannot be finalized is a failure with that
// reason, never "saved", even when part of the file exists.
func TestStopFailureIsReported(t *testing.T) {
	logged := logtest.Tee(t)
	engine := &recordertest.Engine{WriteOutput: true, StopErr: errors.New("recorder: finalize: no space left on device")}
	s, log := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Stop()
	waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, "the failure notice")
	wantFailure(t, "a failed stop", log, logged, i18n.T("recorder-toast-failed"), "no space left")
	if snap := s.Snapshot(); snap.Status != recorder.StatusIdle || snap.Error != i18n.T("recorder-toast-failed") {
		t.Errorf("after a failed stop = %+v", snap)
	}
}

func TestUnexpectedTerminationTearsDown(t *testing.T) {
	engine := &recordertest.Engine{}
	s, log := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	rec := engine.Recordings()[0]
	rec.Fail(errors.New("disk full"))
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
	if rec.Stops() != 1 {
		t.Errorf("stops = %d", rec.Stops())
	}
	if _, n := log.snapshot(); len(n) != 1 || n[0] != i18n.T("recorder-toast-failed")+": " {
		t.Errorf("notices = %q, want the summary without the reason", n)
	}
	// A stale report from a finished recording is ignored.
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "recording again")
	rec.Fail(errors.New("late")) // the report is handled before Fail returns
	if !s.Snapshot().Active {
		t.Error("a stale termination stopped the new recording")
	}
	s.Stop()
}

func TestElapsedSkipsPausedSeconds(t *testing.T) {
	engine := &recordertest.Engine{}
	s, _ := newState(t, engine, nil)
	ticks := make(chan time.Time)
	recorder.DriveElapsed(s, ticks)
	// tick hands the timer a tick; the timer takes the next one only
	// once it handled this one.
	tick := func() {
		t.Helper()
		waittest.Until(t, func() bool {
			select {
			case ticks <- time.Time{}:
				return true
			default:
				return false
			}
		}, "the elapsed timer to take a tick")
	}
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	tick()
	tick()
	waittest.Until(t, func() bool { return s.Snapshot().ElapsedSecs == 2 }, "two seconds")
	s.SetPaused(true)
	tick()
	tick()
	tick() // the first two are handled, paused
	if got := s.Snapshot().ElapsedSecs; got != 2 {
		t.Errorf("elapsed moved while paused: 2 -> %d", got)
	}
	s.SetPaused(false)
	tick()
	waittest.Until(t, func() bool { return s.Snapshot().ElapsedSecs >= 3 }, "a second after the resume")
	s.Stop()
}

func TestToggle(t *testing.T) {
	engine := &recordertest.Engine{}
	s, _ := newState(t, engine, nil)
	s.Toggle()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Toggle()
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
}

func TestFormatElapsed(t *testing.T) {
	for secs, want := range map[uint32]string{5: "0:05", 65: "1:05", 3661: "1:01:01", 0: "0:00"} {
		if got := recorder.FormatElapsed(secs); got != want {
			t.Errorf("FormatElapsed(%d) = %q, want %q", secs, got, want)
		}
	}
}

func TestOutputPath(t *testing.T) {
	t.Setenv("XDG_VIDEOS_DIR", "")
	t.Setenv("HOME", "/home/u")
	now := time.Date(2026, 10, 1, 9, 8, 7, 0, time.Local)
	if got := recorder.OutputPath("", config.RecorderFormatMkv, now); got != "/home/u/Videos/lys-shell-20261001-090807.mkv" {
		t.Errorf("default = %q", got)
	}
	if got := recorder.OutputPath("/custom", config.RecorderFormatMp4, now); got != "/custom/lys-shell-20261001-090807.mp4" {
		t.Errorf("configured = %q", got)
	}
	t.Setenv("XDG_VIDEOS_DIR", "/vids")
	if got := recorder.VideosDir(); got != "/vids" {
		t.Errorf("env = %q", got)
	}
}

func TestBuildOptionsWithoutWebcam(t *testing.T) {
	cfg := config.Defaults().Recorder
	cfg.WebcamEnabled = false
	if opts := recorder.BuildOptions(cfg, time.Now()); opts.Webcam != nil {
		t.Error("webcam disabled still overlays")
	}
}

func TestSessionCloseFailureFailsTheStoppedRecording(t *testing.T) {
	logged := logtest.Tee(t)
	closeErr := errors.New("recorder: close the compositor connection capturing DP-1: broken pipe")
	engine := &recordertest.Engine{WriteOutput: true, CloseErr: closeErr}
	s, log := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	s.Stop()
	waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, "the failure notice")
	toasts, _ := log.snapshot()
	wantFailure(t, "a failed close", log, logged, i18n.T("recorder-toast-failed"), closeErr.Error())
	if slices.Contains(toasts, i18n.T("recorder-toast-stopped")) {
		t.Errorf("toasts = %q: a recording whose session failed to close was reported saved", toasts)
	}
	if snap := s.Snapshot(); snap.Status != recorder.StatusIdle || snap.Error != i18n.T("recorder-toast-failed") {
		t.Errorf("after the failed close = %+v", snap)
	}
	if got := engine.Sessions()[0].Closed(); got != 1 {
		t.Errorf("session closes %d, want 1", got)
	}
}

func TestSessionCloseFailureJoinsTheFailedStart(t *testing.T) {
	startErr := errors.New("recorder: a 64x48 H.264 encoder: out of memory")
	closeErr := errors.New("recorder: close the compositor connection capturing DP-1: broken pipe")
	engine := &recordertest.Engine{StartErr: startErr, CloseErr: closeErr}
	logged := logtest.Tee(t)
	s, log := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { _, n := log.snapshot(); return len(n) == 1 }, "the failure notice")
	if snap := s.Snapshot(); snap.Status != recorder.StatusIdle || snap.Error != i18n.T("recorder-toast-failed") {
		t.Errorf("after the failed start = %+v, want idle with the summary", snap)
	}
	if got := logged.String(); !strings.Contains(got, startErr.Error()) || !strings.Contains(got, closeErr.Error()) {
		t.Errorf("log = %q, want both the start and the close error", got)
	}
}

func TestSessionCloseFailureOfATerminatedRecordingIsLogged(t *testing.T) {
	logged := logtest.Tee(t)
	closeErr := errors.New("recorder: close the compositor connection capturing DP-1: broken pipe")
	engine := &recordertest.Engine{CloseErr: closeErr}
	s, _ := newState(t, engine, nil)
	s.Start()
	waittest.Until(t, func() bool { return s.Snapshot().Active }, "the state to be recording")
	engine.Recordings()[0].Fail(errors.New("disk full"))
	waittest.Until(t, func() bool { return s.Snapshot().Status == recorder.StatusIdle }, "the state to be idle")
	if got := logged.String(); !strings.Contains(got, "end the terminated recording: "+closeErr.Error()) {
		t.Errorf("log = %q, want the close error of the terminated recording", got)
	}
	if got := s.Snapshot().Error; got != i18n.T("recorder-toast-failed") {
		t.Errorf("error state %q, want the summary", got)
	}
	if got := logged.String(); !strings.Contains(got, "terminated unexpectedly: disk full") {
		t.Errorf("log = %q, want the termination reason", got)
	}
}
