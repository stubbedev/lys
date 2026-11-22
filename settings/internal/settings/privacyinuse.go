package settings

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/shell/service/screenshare"
	"github.com/stubbedev/lys/ui/widget"

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
	// unread are the summaries of the sources that could not be read,
	// and failures the errors why, each wrapped in its summary.
	unread   []string
	failures []error
}

// miss records that the source summaryKey names could not be read.
func (s *captureSources) miss(summaryKey string, err error) {
	summary := locale.T().Get(summaryKey)
	s.unread = append(s.unread, summary)
	s.failures = append(s.failures, fmt.Errorf("%s: %w", summary, err))
}

func openCaptureSources(k *kit) captureSources {
	var s captureSources
	ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
	defer cancel()
	if svc, err := openPulse(ctx); err != nil {
		s.miss("settings-privacy-in-use-no-microphone", err)
	} else {
		s.svc = svc
		s.releases = append(s.releases, func() { _ = svc.Close() })
	}
	if cameras, release, err := k.cameras(); err != nil {
		s.miss("settings-privacy-in-use-no-camera", err)
	} else {
		s.cameras = cameras
		s.releases = append(s.releases, release)
	}
	if shares, release, err := k.screenShares(); err != nil {
		s.miss("settings-privacy-in-use-no-screen", err)
	} else {
		s.shares = shares
		s.releases = append(s.releases, release)
	}
	return s
}

type inUseStrip struct {
	*widget.Box
	l     *grantsList
	group *widget.ItemGroup
	state *cardState
	src   captureSources
	read  bool
	// failure is why the last change to a capture failed, nil once one
	// did not.
	failure error
	stops   []func()
	ticked  atomic.Bool
	closed  bool
}

func newInUseStrip(l *grantsList) *inUseStrip {
	k := l.k
	s := &inUseStrip{l: l, group: widget.NewItemGroup(), state: k.cardState()}
	s.Box = newSection(k, "settings-section-privacy-in-use", s.group)
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
	s.src, s.read = src, true
	for _, err := range src.failures {
		log.Printf("settings: %v", err)
	}
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
	s.group.Append(s.state)
	n := 0
	if svc := s.src.svc; svc != nil {
		for _, u := range privacy.MicUsers(svc.RecordingStreams(), svc.InputDevices(), intconv.Clamp[uint32](os.Getpid())) {
			row := k.row(u.Name, t.Get("settings-privacy-in-use-microphone", i18n.Str("source", u.Source)))
			row.SetMedia(widget.NewThemeIconFallback(cmp.Or(u.Icon, appFallbackIcon), appIconPx), true)
			key := u.Stream
			mute := newMuteToggle(k, micGlyphs, func(on bool) { s.mute(svc, key, on) })
			mute.name(t.Get("settings-sound-app-mute", i18n.Str("app", u.Name)))
			mute.show(u.Muted)
			row.withControl(mute)
			s.group.Append(row)
			n++
		}
	}
	if s.src.cameras != nil {
		for _, u := range s.src.cameras.Users() {
			row := k.row(u.Name, t.Get("settings-privacy-in-use-camera", i18n.Str("device", u.Device)))
			row.SetMedia(k.icon(cameraResource.icon), false)
			s.group.Append(row)
			n++
		}
	}
	if shares := s.src.shares; shares != nil {
		for _, sh := range shares.List() {
			title := s.l.appTitle(sh.App)
			row := k.row(title, t.Get("settings-privacy-in-use-screen"))
			row.SetMedia(widget.NewThemeIcon(s.l.appIcon(sh.App), appIconPx), true)
			session := sh.Session
			var stop *widget.Button
			stop, _ = k.contentButton(roleSecondary, "ld-x-symbolic", t.Get("settings-privacy-in-use-stop"), func() {
				stop.SetEnabled(false)
				s.stopShare(shares, session)
			})
			stop.SetAccessibleName(t.Get("settings-privacy-in-use-stop-app", i18n.Str("app", title)))
			row.withControl(stop)
			s.group.Append(row)
			n++
		}
	}
	s.showState(n)
}

// showState shows, in the strip's one state, a change that failed, the
// sources that could not be read, or that n, the uses listed, is none.
func (s *inUseStrip) showState(n int) {
	t := locale.T()
	switch {
	case !s.read:
		s.state.Loading(t.Get("settings-privacy-in-use-reading"))
	case s.failure != nil:
		s.state.Error(t.Get("settings-privacy-in-use-failed"), s.failure, nil)
	case len(s.src.unread) == 1:
		s.state.Unavailable("ld-shield-check-symbolic", s.src.unread[0], s.src.failures[0])
	case len(s.src.unread) > 1:
		s.state.Unavailable("ld-shield-check-symbolic", t.Get("settings-privacy-in-use-unreadable"), errors.Join(s.src.failures...))
	case n == 0:
		s.state.Empty("ld-shield-check-symbolic", t.Get("settings-privacy-in-use-none"))
	default:
		s.state.Clear()
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

// failed shows why a change to a capture failed, or clears what failed
// before for nil.
func (s *inUseStrip) failed(err error) {
	if err != nil {
		log.Printf("settings: in use now: %v", err)
	}
	if err == nil && s.failure == nil {
		return
	}
	s.failure = err
	if !s.closed {
		s.render()
	}
}
