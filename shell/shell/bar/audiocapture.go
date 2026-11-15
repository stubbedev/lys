package bar

import (
	"context"
	"log"
	"os"
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
// per process holding a camera, with a button that ends it; and per
// screen share, with its Stop. Hidden while nothing captures.
type captureSection struct {
	*widget.Box
	ctx  ModuleContext
	font render.Font
	px   float64
	list *widget.ItemGroup
	// signal ends a camera's process (syscall.Kill; a test seam).
	signal func(pid int, sig syscall.Signal) error
}

func newCaptureSection(ctx ModuleContext, font render.Font, px float64) *captureSection {
	c := &captureSection{ctx: ctx, font: font, px: px, signal: syscall.Kill}
	card, list := itemCard()
	card.AddClass("audio-capture-list")
	c.list = list
	c.Box = dropdownSection(sectionLabel(font, i18n.T("dropdown-audio-capture")), card)
	c.AddClass("audio-capture")
	c.SetVisible(false)
	return c
}

// apply lists what captures now.
func (c *captureSection) apply(s captureState) {
	c.list.Clear()
	c.SetVisible(!s.idle())
	cfg := c.ctx.Config.Audio
	for _, u := range s.mics {
		key, muted := u.Stream, u.Muted
		label := "dropdown-privacy-mute"
		if muted {
			label = "dropdown-privacy-unmute"
		}
		c.list.Append(c.row(u.Name, i18n.T("dropdown-privacy-mic", i18n.Str("source", u.Source)), cfg.MicIcon, i18n.T(label), func() {
			c.ctx.Go(func() {
				if err := c.ctx.Pulse.SetStreamMute(context.Background(), key, !muted); err != nil {
					log.Printf("audio: mute %s: %v", u.Name, err)
				}
			})
		}))
	}
	for _, cam := range s.cameras {
		pid := cam.PID
		c.list.Append(c.row(cam.Name, i18n.T("dropdown-privacy-camera", i18n.Str("device", cam.Device), i18n.Str("pid", strconv.Itoa(pid))), cfg.CaptureCameraIcon, i18n.T("dropdown-privacy-end"), func() {
			if err := c.signal(pid, syscall.SIGTERM); err != nil {
				log.Printf("audio: end %d: %v", pid, err)
			}
		}))
	}
	for _, sh := range s.shares {
		session := sh.Session
		c.list.Append(c.row(shareApp(sh), shareTargets(sh), cfg.CaptureScreenIcon, i18n.T("dropdown-screen-share-stop"), func() {
			c.ctx.Go(func() {
				if err := c.ctx.ScreenShares.Stop(context.Background(), session); err != nil {
					log.Printf("audio: stop screen share %s: %v", session, err)
				}
			})
		}))
	}
}

// row is one app: its name over what it uses, and its action.
func (c *captureSection) row(title, subtitle, icon, action string, run func()) *widget.Item {
	row := widget.NewItem(c.font, title, subtitle)
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("audio-capture-row")
	row.SetMedia(widget.NewThemeIcon(icon, itemIconPx), false)
	row.AddAction(dropdownButton(widget.NewLabel(c.font, c.px, action, 0), run, "audio-capture-action", css.DestructiveAction, css.Small))
	return row
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
