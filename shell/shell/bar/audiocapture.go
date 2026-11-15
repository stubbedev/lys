package bar

import (
	"context"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/shell/service/screenshare"
)

// cameraSource is the processes holding a camera (*privacy.Cameras).
type cameraSource interface {
	Users() []privacy.CameraUser
	Subscribe() (<-chan struct{}, func())
}

// shareSource is the portal's screen shares (*screenshare.Shares).
type shareSource interface {
	List() []screenshare.Share
	Subscribe() (<-chan struct{}, func())
	Stop(ctx context.Context, session dbus.ObjectPath) error
}

// captureState is what captures the user now: the apps recording a
// microphone, the processes holding a camera, the screen shares.
type captureState struct {
	mics    []privacy.MicUser
	cameras []privacy.CameraUser
	shares  []screenshare.Share
}

// idle reports that nothing captures the user.
func (s captureState) idle() bool { return len(s.mics)+len(s.cameras)+len(s.shares) == 0 }

// selfPID is lys-shell's own process, whose recorder and visualizer
// record without counting as an app listening in.
func selfPID() uint32 { return intconv.Clamp[uint32](os.Getpid()) }

// readCapture reads the state from the sources there are.
func readCapture(ctx ModuleContext) captureState {
	var s captureState
	if ctx.Pulse != nil {
		s.mics = privacy.MicUsers(ctx.Pulse.RecordingStreams(), ctx.Pulse.InputDevices(), selfPID())
	}
	if ctx.Cameras != nil {
		s.cameras = ctx.Cameras.Users()
	}
	if ctx.ScreenShares != nil {
		s.shares = ctx.ScreenShares.List()
	}
	return s
}

// captureFeeds are the change feeds of the camera and screen-share
// sources there are; the recording streams change with the audio
// source, which its reader follows already.
func captureFeeds(ctx ModuleContext) []subscription {
	var subs []subscription
	if ctx.Cameras != nil {
		subs = append(subs, ctx.Cameras.Subscribe)
	}
	if ctx.ScreenShares != nil {
		subs = append(subs, ctx.ScreenShares.Subscribe)
	}
	return subs
}

// captureSection is the audio dropdown's capture card: a row per app
// recording the microphone, with a button that mutes that stream alone;
// per process holding a camera, with a button that ends it once the row
// confirms; and per screen share, with its Stop. Hidden while nothing
// captures.
type captureSection struct {
	*widget.Box
	ctx  ModuleContext
	font render.Font
	px   float64
	list *widget.ItemGroup
	// signal ends a camera's process (syscall.Kill; a test seam).
	signal func(pid int, sig syscall.Signal) error

	last       captureState
	prompt     *cameraPrompt
	shareStops *stopButtons[dbus.ObjectPath]
}

type cameraPrompt struct {
	pid     int
	failure error
}

func newCaptureSection(ctx ModuleContext, font render.Font, px float64, refresh func()) *captureSection {
	c := &captureSection{ctx: ctx, font: font, px: px, signal: syscall.Kill, shareStops: newStopButtons[dbus.ObjectPath](ctx, "audio", refresh)}
	card, list := itemCard()
	card.AddClass("audio-capture-list")
	c.list = list
	c.Box = dropdownSection(sectionLabel(font, i18n.T("dropdown-audio-capture")), card)
	c.AddClass("audio-capture")
	c.SetVisible(false)
	return c
}

// apply lists what captures now; a camera's prompt ends with its row.
func (c *captureSection) apply(s captureState) {
	c.last = s
	if c.prompt != nil && !slices.ContainsFunc(s.cameras, func(cam privacy.CameraUser) bool { return cam.PID == c.prompt.pid }) {
		c.prompt = nil
	}
	c.render()
}

func (c *captureSection) render() {
	s := c.last
	c.list.Clear()
	c.SetVisible(!s.idle())
	cfg := c.ctx.Config.Audio
	for _, u := range s.mics {
		key, muted := u.Stream, u.Muted
		label := "dropdown-privacy-mute"
		if muted {
			label = "dropdown-privacy-unmute"
		}
		row := c.row(u.Name, i18n.T("dropdown-privacy-mic", i18n.Str("source", u.Source)), cfg.MicIcon)
		row.AddAction(c.button(i18n.T(label), func() {
			c.ctx.Go(func() {
				if err := c.ctx.Pulse.SetStreamMute(context.Background(), key, !muted); err != nil {
					log.Printf("audio: mute %s: %v", u.Name, err)
				}
			})
		}, "audio-capture-action", css.Outline))
		c.list.Append(row)
	}
	for _, cam := range s.cameras {
		c.list.Append(c.cameraRow(cam, cfg.CaptureCameraIcon))
	}
	for _, sh := range s.shares {
		session, app := sh.Session, shareApp(sh)
		row := c.row(app, shareTargets(sh), cfg.CaptureScreenIcon)
		row.AddAction(c.shareStops.button(c.font, c.px, i18n.T("dropdown-screen-share-stop"), session, "audio-capture-action",
			func(err error) string {
				return i18n.T("dropdown-screen-share-stop-failed", i18n.Str("app", app), i18n.Str("error", err.Error()))
			},
			func(ctx context.Context) error { return c.ctx.ScreenShares.Stop(ctx, session) }))
		c.list.Append(row)
	}
}

func (c *captureSection) cameraRow(cam privacy.CameraUser, icon string) *widget.Item {
	pid := cam.PID
	ask := func() {
		c.prompt = &cameraPrompt{pid: pid}
		c.render()
	}
	if c.prompt == nil || c.prompt.pid != pid {
		row := c.row(cam.Name, cameraUse(cam), icon)
		row.AddAction(c.button(i18n.T("dropdown-privacy-end"), ask, "audio-capture-action", css.DestructiveAction))
		return row
	}
	if err := c.prompt.failure; err != nil {
		row := c.row(cam.Name, i18n.T("dropdown-privacy-end-failed", i18n.Str("app", cam.Name), i18n.Str("error", err.Error())), icon)
		row.AddClass("audio-capture-failed", css.Error)
		row.AddAction(c.button(i18n.T("dropdown-privacy-end"), ask, "audio-capture-action", css.DestructiveAction))
		return row
	}
	row := c.row(cam.Name, i18n.T("dropdown-privacy-end-confirm", i18n.Str("app", cam.Name)), icon)
	row.AddClass("audio-capture-confirming")
	row.AddAction(c.button(i18n.T("dropdown-privacy-cancel"), func() {
		c.prompt = nil
		c.render()
	}, "audio-capture-cancel", css.Outline))
	row.AddAction(c.button(i18n.T("dropdown-privacy-end"), func() {
		c.prompt = nil
		if err := c.signal(pid, syscall.SIGTERM); err != nil {
			log.Printf("audio: end %s (pid %d): %v", cam.Name, pid, err)
			c.prompt = &cameraPrompt{pid: pid, failure: err}
		}
		c.render()
	}, "audio-capture-confirm", css.DestructiveAction))
	return row
}

func cameraUse(cam privacy.CameraUser) string {
	return i18n.T("dropdown-privacy-camera", i18n.Str("device", cam.Device), i18n.Str("pid", strconv.Itoa(cam.PID)))
}

// row is one app: its name over what it uses.
func (c *captureSection) row(title, subtitle, icon string) *widget.Item {
	row := widget.NewItem(c.font, title, subtitle)
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("audio-capture-row")
	row.SetMedia(widget.NewThemeIcon(icon, itemIconPx), false)
	return row
}

func (c *captureSection) button(label string, run func(), classes ...string) *widget.Button {
	return dropdownButton(widget.NewLabel(c.font, c.px, label, 0), run, append(classes, css.Small)...)
}

type stopButtons[K comparable] struct {
	ctx     ModuleContext
	module  string
	pending map[K]struct{}
	done    func()
}

func newStopButtons[K comparable](ctx ModuleContext, module string, done func()) *stopButtons[K] {
	return &stopButtons[K]{ctx: ctx, module: module, pending: map[K]struct{}{}, done: done}
}

func (s *stopButtons[K]) button(font render.Font, px float64, label string, key K, class string, failed func(error) string, stop func(context.Context) error) *widget.Button {
	b := dropdownButton(widget.NewLabel(font, px, label, 0), nil, class, css.DestructiveAction, css.Small)
	_, running := s.pending[key]
	b.SetEnabled(!running)
	b.OnClick = func() {
		s.pending[key] = struct{}{}
		b.SetEnabled(false)
		s.ctx.Go(func() {
			life, cancel := context.WithTimeout(s.ctx.Life(), offLoopTimeout)
			err := stop(life)
			cancel()
			s.ctx.Invoke(func() {
				delete(s.pending, key)
				if err != nil {
					log.Printf("%s: %v", s.module, err)
					if s.ctx.Toast != nil {
						s.ctx.Toast(failed(err), "ld-alert-triangle-symbolic")
					}
				}
				s.done()
			})
		})
	}
	return b
}

// shareApp names the app sharing: its id, or "An app" for one outside a
// sandbox, which the portal knows no id of.
func shareApp(s screenshare.Share) string {
	app := s.App
	if app == "" {
		app = i18n.T("dropdown-screen-share-unknown-app")
	}
	if s.Kind == screenshare.KindRemoteDesktop {
		return i18n.T("dropdown-screen-share-remote", i18n.Str("app", app))
	}
	return app
}

// shareTargets says what a share streams: each screen by its connector,
// each window by its title.
func shareTargets(s screenshare.Share) string {
	parts := make([]string, 0, len(s.Targets))
	for _, t := range s.Targets {
		switch t.Kind {
		case screenshare.TargetWindow:
			title := t.Title
			if title == "" {
				title = t.Name
			}
			parts = append(parts, i18n.T("dropdown-screen-share-window", i18n.Str("title", title)))
		case screenshare.TargetRegion:
			parts = append(parts, i18n.T("dropdown-screen-share-region", i18n.Str("name", t.Name)))
		case screenshare.TargetDynamic:
			parts = append(parts, i18n.T("dropdown-screen-share-dynamic"))
		default:
			parts = append(parts, i18n.T("dropdown-screen-share-output", i18n.Str("name", t.Name)))
		}
	}
	return strings.Join(parts, ", ")
}
