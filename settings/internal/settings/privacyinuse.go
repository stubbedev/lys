package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/shell/service/screenshare"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type cameraSource interface {
	Users() []privacy.CameraUser
	Subscribe() (<-chan struct{}, func())
}

type shareSource interface {
	List() []screenshare.Share
	Subscribe() (<-chan struct{}, func())
	Stop(ctx context.Context, session dbus.ObjectPath) error
}

func watchCameras() (cameraSource, func(), error) {
	c, err := privacy.WatchCameras("/dev", "/proc", os.Getuid(), desktopentry.ApplicationDirs())
	if err != nil {
		return nil, nil, err
	}
	return c, func() {
		if err := c.Close(); err != nil {
			log.Printf("settings: stop following the cameras: %v", err)
		}
	}, nil
}

func followShares(dial func() (*dbus.Conn, error)) (shareSource, func(), error) {
	conn, err := dial()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	shares, err := screenshare.Follow(conn)
	if err != nil {
		return nil, nil, errors.Join(fmt.Errorf("follow the screen shares: %w", err), conn.Close())
	}
	return shares, func() {
		shares.Close()
		_ = conn.Close()
	}, nil
}

type captureSources struct {
	svc      *pulse.Service
	cameras  cameraSource
	shares   shareSource
	releases []func()
	problems []string
}

func openCaptureSources(k *kit) captureSources {
	t := locale.T()
	var s captureSources
	ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
	defer cancel()
	if svc, err := openPulse(ctx); err != nil {
		s.problems = append(s.problems, t.Get("settings-privacy-in-use-no-microphone")+": "+err.Error())
	} else {
		s.svc = svc
		s.releases = append(s.releases, func() { _ = svc.Close() })
	}
	if cameras, release, err := k.cameras(); err != nil {
		s.problems = append(s.problems, t.Get("settings-privacy-in-use-no-camera")+": "+err.Error())
	} else {
		s.cameras = cameras
		s.releases = append(s.releases, release)
	}
	if shares, release, err := k.screenShares(); err != nil {
		s.problems = append(s.problems, t.Get("settings-privacy-in-use-no-screen")+": "+err.Error())
	} else {
		s.shares = shares
		s.releases = append(s.releases, release)
	}
	return s
}

type inUseStrip struct {
	*widget.Box
	l       *grantsList
	group   *widget.ItemGroup
	problem *widget.Label
	src     captureSources
	stops   []func()
	ticked  atomic.Bool
	closed  bool
}

func newInUseStrip(l *grantsList) *inUseStrip {
	k := l.k
	s := &inUseStrip{l: l, group: widget.NewItemGroup()}
	s.problem = wrapped(k.label("", css.Caption, css.DimLabel))
	s.problem.SetVisible(false)
	body := gapBox(widget.Column)
	body.Append(s.group, false)
	body.Append(s.problem, false)
	s.Box = newSection(k, "settings-section-privacy-in-use", body)
	s.render()
	async(k, func() captureSources { return openCaptureSources(k) }, s.opened)
	return s
}

func (s *inUseStrip) opened(src captureSources) {
	if s.closed {
		for _, release := range src.releases {
			release()
		}
		return
	}
	s.src = src
	for _, p := range src.problems {
		log.Printf("settings: %s", p)
	}
	s.problem.SetText(strings.Join(src.problems, "\n"))
	s.problem.SetVisible(len(src.problems) > 0)
	if src.svc != nil {
		ticks, stop, err := src.svc.Subscribe(context.Background())
		if err != nil {
			log.Printf("settings: follow the recording streams: %v", err)
		} else {
			s.follow(ticks, stop)
		}
	}
	if src.cameras != nil {
		s.follow(src.cameras.Subscribe())
	}
	if src.shares != nil {
		s.follow(src.shares.Subscribe())
	}
	s.render()
}

func (s *inUseStrip) follow(ticks <-chan struct{}, stop func()) {
	s.stops = append(s.stops, stop)
	go func() {
		for range ticks {
			if s.ticked.CompareAndSwap(false, true) {
				s.l.k.invoke(s.tick)
			}
		}
	}()
}

func (s *inUseStrip) tick() {
	s.ticked.Store(false)
	if !s.closed {
		s.render()
	}
}

func (s *inUseStrip) close() {
	s.closed = true
	for _, stop := range s.stops {
		stop()
	}
	s.stops = nil
	for _, release := range s.src.releases {
		release()
	}
	s.src = captureSources{}
}

func (s *inUseStrip) render() {
	t := locale.T()
	k := s.l.k
	s.group.Clear()
	n := 0
	if svc := s.src.svc; svc != nil {
		for _, u := range privacy.MicUsers(svc.RecordingStreams(), svc.InputDevices(), intconv.Clamp[uint32](os.Getpid())) {
			row := widget.NewItem(k.face, u.Name, t.Get("settings-privacy-in-use-microphone", i18n.Str("source", u.Source)))
			row.SetMedia(widget.NewThemeIconFallback(cmp.Or(u.Icon, appFallbackIcon), appIconPx), true)
			key := u.Stream
			mute := newMuteToggle(k, micGlyphs, func(on bool) { s.mute(svc, key, on) })
			mute.name(t.Get("settings-sound-app-mute", i18n.Str("app", u.Name)))
			mute.show(u.Muted)
			row.AddAction(mute)
			s.group.Append(row)
			n++
		}
	}
	if s.src.cameras != nil {
		for _, u := range s.src.cameras.Users() {
			row := widget.NewItem(k.face, u.Name, t.Get("settings-privacy-in-use-camera", i18n.Str("device", u.Device)))
			row.SetMedia(k.icon(cameraResource.icon), false)
			s.group.Append(row)
			n++
		}
	}
	if shares := s.src.shares; shares != nil {
		for _, sh := range shares.List() {
			title := s.l.appTitle(sh.App)
			row := widget.NewItem(k.face, title, t.Get("settings-privacy-in-use-screen"))
			row.SetMedia(widget.NewThemeIcon(s.l.appIcon(sh.App), appIconPx), true)
			session := sh.Session
			var stop *widget.Button
			stop, _ = k.contentButton(roleSecondary, "ld-x-symbolic", t.Get("settings-privacy-in-use-stop"), func() {
				stop.SetEnabled(false)
				s.stopShare(shares, session)
			})
			stop.SetAccessibleName(t.Get("settings-privacy-in-use-stop-app", i18n.Str("app", title)))
			row.AddAction(stop)
			s.group.Append(row)
			n++
		}
	}
	if n == 0 {
		s.group.Append(k.emptyRow("ld-shield-check-symbolic", t.Get("settings-privacy-in-use-none")))
	}
}

func (s *inUseStrip) mute(svc *pulse.Service, key pulse.StreamKey, on bool) {
	async(s.l.k, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		return svc.SetStreamMute(ctx, key, on)
	}, s.failed)
}

func (s *inUseStrip) stopShare(shares shareSource, session dbus.ObjectPath) {
	async(s.l.k, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), grantsTimeout)
		defer cancel()
		return shares.Stop(ctx, session)
	}, s.failed)
}

func (s *inUseStrip) failed(err error) {
	if err == nil {
		return
	}
	log.Printf("settings: in use now: %v", err)
	s.problem.SetText(locale.T().Get("settings-privacy-in-use-failed") + ": " + err.Error())
	s.problem.SetVisible(true)
	if !s.closed {
		s.render()
	}
}
