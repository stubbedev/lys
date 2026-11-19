package bar

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
)

// volumePercent is the level the module displays: the channel average
// as a percentage, rounded.
func volumePercent(dev pulse.Device) float64 {
	return math.Round(dev.Volume.AveragePercentage())
}

// audioRead is one read of what the audio module shows: the default
// output and input (nil without one) and what captures the user.
type audioRead struct {
	speaker, mic *pulse.Device
	capture      captureState
}

// audioFace is what the button shows for one read.
type audioFace struct {
	visible      bool
	speakerShown bool
	speakerIcon  string
	label        string
	// level is the speaker percentage the thresholds read; hasLevel is
	// false without a speaker.
	level    float64
	hasLevel bool
	micShown bool
	micIcon  string
	// micCapturing lights the microphone icon: an app records it.
	micCapturing bool
	cameraShown  bool
	screenShown  bool
	description  string
}

// audioFaceOf is the face cfg gives r. The module shows while there is
// a speaker, a microphone or anything capturing.
func audioFaceOf(cfg config.AudioConfig, r audioRead) audioFace {
	f := audioFace{
		micCapturing: len(r.capture.mics) > 0,
		cameraShown:  len(r.capture.cameras) > 0,
		screenShown:  len(r.capture.shares) > 0,
	}
	var state []string
	f.micIcon = cfg.MicIcon
	if r.mic != nil && r.mic.Muted {
		f.micIcon = cfg.MicIconMuted
	}
	f.micShown = f.micCapturing || (r.mic != nil && !cfg.MicHideWhenIdle)
	if r.speaker != nil {
		f.level, f.hasLevel = volumePercent(*r.speaker), true
		f.speakerShown = cfg.IconShow
		f.speakerIcon = speakerIconName(cfg, *r.speaker)
		f.label = jinja.RenderOr(cfg.Format, audioVars(*r.speaker, r.mic))
		level := strconv.Itoa(int(f.level)) + "%"
		if r.speaker.Muted {
			level = i18n.T("bar-audio-speaker-muted", i18n.Str("level", level))
		}
		state = append(state, level)
	}
	if r.mic != nil && r.mic.Muted {
		state = append(state, i18n.T("bar-audio-mic-muted"))
	}
	for _, c := range []struct {
		on  bool
		key string
	}{{f.micCapturing, "bar-audio-capture-mic"}, {f.cameraShown, "bar-audio-capture-camera"}, {f.screenShown, "bar-audio-capture-screen"}} {
		if c.on {
			state = append(state, i18n.T(c.key))
		}
	}
	f.description = strings.Join(state, ", ")
	f.visible = r.speaker != nil || r.mic != nil || !r.capture.idle()
	return f
}

// audioVars are the format's values: the speaker's percent, and the
// microphone's while there is one.
func audioVars(speaker pulse.Device, mic *pulse.Device) map[string]any {
	vars := map[string]any{"percent": int64(volumePercent(speaker))}
	if mic != nil {
		vars["mic_percent"] = int64(volumePercent(*mic))
	}
	return vars
}

// speakerIconName picks the speaker icon: the muted icon wins, then the
// level list spans 1..100 across icons 0..n-1 (0% takes the first),
// over the rounded average.
func speakerIconName(cfg config.AudioConfig, dev pulse.Device) string {
	if dev.Muted || len(cfg.SpeakerIcons) == 0 {
		return cfg.SpeakerIconMuted
	}
	return cfg.SpeakerIcons[levelIndexSpan(int(volumePercent(dev)), len(cfg.SpeakerIcons))]
}

// audioModule is the audio module: the speaker's level and mute, the
// microphone's mute, and the capture indicator, refreshed on audio,
// camera and screen-share changes.
type audioModule struct {
	gate   firstRead
	ctx    ModuleContext
	source pulse.Source
	btn    *barButton

	speaker, mic, camera, screen *widget.Icon
	chrome                       moduleChrome
	face                         audioFace
	// applied is false until the first read shows.
	applied bool
}

func newAudio(ctx ModuleContext) (Module, error) {
	if ctx.Pulse == nil {
		return nil, errors.New("audio: no audio source available")
	}
	m := newAudioModule(ctx, ctx.Pulse)
	reads := followReads(ctx, "audio", &m.gate, infallibleStop(ctx.Pulse.Subscribe), m.read, m.apply)
	for _, feed := range captureFeeds(ctx) {
		ticks, stop := feed()
		reads.on(ticks, stop)
	}
	return m, nil
}

// newAudioModule builds the button around source, before any read.
func newAudioModule(ctx ModuleContext, source pulse.Source) *audioModule {
	cfg := ctx.Config.Audio
	m := &audioModule{ctx: ctx, source: source}
	icons := widget.NewBox(widget.Row, 2, 0)
	icons.AddClass("audio-icons")
	capture := "color: " + cfg.CaptureColor.ToCSS() + ";"
	for _, slot := range []struct {
		icon  **widget.Icon
		name  string
		class string
		style string
	}{
		{&m.speaker, cfg.Icon().Name, "audio-speaker", ""},
		{&m.mic, cfg.MicIcon, "audio-mic", ""},
		{&m.camera, cfg.CaptureCameraIcon, "audio-capture-camera", capture},
		{&m.screen, cfg.CaptureScreenIcon, "audio-capture-screen", capture},
	} {
		icon := widget.NewThemeIcon(slot.name, barview.IconPx)
		icon.AddClass("bar-button-icon", slot.class)
		icon.SetInlineStyle(slot.style)
		icon.SetVisible(false)
		icons.Append(icon, false)
		*slot.icon = icon
	}
	m.btn = newBarButtonAround(ctx, icons)
	return m
}

func (m *audioModule) read(ctx context.Context) (audioRead, error) {
	var r audioRead
	out, err := m.source.DefaultSink(ctx)
	switch {
	case err == nil:
		r.speaker = &out.Device
	case !errors.Is(err, pulse.ErrNoDefaultDevice):
		return audioRead{}, err
	}
	in, err := m.source.DefaultSource(ctx)
	switch {
	case err == nil:
		r.mic = &in.Device
	case !errors.Is(err, pulse.ErrNoDefaultDevice):
		return audioRead{}, err
	}
	r.capture = readCapture(m.ctx)
	return r, nil
}

// apply shows one read.
func (m *audioModule) apply(r audioRead) {
	cfg := m.ctx.Config.Audio
	f := audioFaceOf(cfg, r)
	m.face, m.applied = f, true
	m.speaker.SetVisible(f.speakerShown)
	if f.speakerShown {
		m.speaker.SetThemeName(f.speakerIcon)
	}
	m.mic.SetVisible(f.micShown)
	m.mic.SetThemeName(f.micIcon)
	if f.micCapturing {
		m.mic.AddClass("capturing")
		m.mic.SetInlineStyle("color: " + cfg.CaptureColor.ToCSS() + ";")
	} else {
		m.mic.RemoveClass("capturing")
		m.mic.SetInlineStyle("")
	}
	m.camera.SetVisible(f.cameraShown)
	m.screen.SetVisible(f.screenShown)
	if f.hasLevel {
		m.btn.ApplyThresholds(f.level, cfg.Thresholds)
	} else {
		m.btn.SetThresholds(config.ThresholdColors{})
	}
	m.btn.SetLabel(f.label)
	m.btn.SetDescription(f.description)
	m.showChrome()
}

func (m *audioModule) showChrome() {
	if m.chrome != nil {
		m.chrome.SetVisible(m.applied && m.face.visible)
	}
}

func (m *audioModule) setChrome(c moduleChrome) {
	m.chrome = c
	m.showChrome()
}

func (m *audioModule) Root() widget.Widget { return m.btn }
