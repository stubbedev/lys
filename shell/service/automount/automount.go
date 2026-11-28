// Package automount mounts the filesystems on removable drives as they
// are plugged in, and those already plugged in when it starts, as GVfs's
// volume monitor and pcmanfm's mount_removable do, and posts for each a
// notification that opens it or ejects it. Internal drives are never
// mounted, nor anything on an encrypted container: a locked one waits
// for its passphrase in the file manager, which mounts what it unlocks.
package automount

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/shell/service/udisks"
)

// Notifier posts a notification, and one with buttons whose answer it
// waits for; desktopnotify's Sender is one.
type Notifier interface {
	Send(ctx context.Context, appName, summary, body, appIcon string) (uint32, error)
	Ask(ctx context.Context, appName, summary, body, appIcon string, actions []desktopnotify.Action, timeout time.Duration) (key string, ok bool, err error)
}

// Deps is what the mounter runs on. Enabled is read at every drive
// plugged in, so a reload applies to the next; Open shows a mounted
// volume's directory in the file manager.
type Deps struct {
	Drives   *udisks.Client
	Notifier Notifier
	Enabled  func() bool
	Open     func(dir string)
}

// ActionEject is the mount notification's eject button; its body, the
// spec's default action, opens the volume.
const ActionEject = "eject"

// Icon is the icon every notification of the mounter carries.
const Icon = "ld-usb-symbolic"

const (
	// callTimeout bounds listing the volumes.
	callTimeout = 5 * time.Second
	// mountTimeout bounds a mount or an eject, which wait on the drive.
	mountTimeout = time.Minute
	// noticeExpiry leaves the notification's lifetime to the daemon.
	noticeExpiry = -time.Millisecond
)

type mounter struct {
	Deps
	life context.Context
	// mu keeps the startup pass and a drive plugged in meanwhile from
	// mounting one volume twice.
	mu     sync.Mutex
	asking sync.WaitGroup
}

// Start mounts the removable volumes present now, unless Enabled says
// otherwise, then each one plugged in until stop, which returns once
// every notification it posted has been answered or abandoned.
func Start(d Deps) (stop func(), err error) {
	life, cancel := context.WithCancel(context.Background())
	m := &mounter{Deps: d, life: life}
	stopWatch, err := d.Drives.WatchFilesystems(m.added)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("automount: %w", err)
	}
	m.asking.Go(func() {
		if m.Enabled() {
			m.mount(func(udisks.Volume) bool { return true })
		}
	})
	return func() {
		stopWatch()
		cancel()
		m.asking.Wait()
	}, nil
}

func (m *mounter) added(object dbus.ObjectPath) {
	if !m.Enabled() {
		return
	}
	m.mount(func(v udisks.Volume) bool { return v.Object == object })
}

// Mountable reports a volume the mounter mounts: a filesystem on a
// removable drive, not mounted yet and on no encrypted container.
func Mountable(v udisks.Volume) bool {
	return v.Removable && !v.Encrypted() && !v.Mounted()
}

// mount mounts every Mountable volume pick picks and announces each.
func (m *mounter) mount(pick func(udisks.Volume) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	vols, err := m.volumes()
	if err != nil {
		log.Printf("automount: %v", err)
		return
	}
	for _, v := range vols {
		if !pick(v) || !Mountable(v) {
			continue
		}
		at, err := m.mountOne(v)
		if err != nil {
			m.fail(i18n.T("file-chooser-mount-failed", volumeArg(v)), err)
			continue
		}
		m.asking.Go(func() { m.announce(v, at) })
	}
}

func (m *mounter) volumes() ([]udisks.Volume, error) {
	call, cancel := context.WithTimeout(m.life, callTimeout)
	defer cancel()
	return m.Drives.Volumes(call)
}

func (m *mounter) mountOne(v udisks.Volume) (string, error) {
	call, cancel := context.WithTimeout(m.life, mountTimeout)
	defer cancel()
	return m.Drives.Mount(call, v.Object)
}

// announce posts the notification for v mounted at at and does what
// its answer asks.
func (m *mounter) announce(v udisks.Volume, at string) {
	key, ok, err := m.Notifier.Ask(m.life, i18n.T("volume-notification-source"), i18n.T("volume-mounted", volumeArg(v)), at, Icon, []desktopnotify.Action{
		{Key: notifications.DefaultActionID, Label: i18n.T("volume-action-open")},
		{Key: ActionEject, Label: i18n.T("volume-action-eject")},
	}, noticeExpiry)
	if err != nil {
		log.Printf("automount: notify %s mounted at %s: %v", v.Object, at, err)
		return
	}
	if !ok {
		return
	}
	switch key {
	case notifications.DefaultActionID:
		m.Open(at)
	case ActionEject:
		m.eject(v)
	}
}

// eject ejects v as it is now: a volume already gone is nothing to do.
func (m *mounter) eject(v udisks.Volume) {
	vols, err := m.volumes()
	if err != nil {
		m.fail(i18n.T("file-chooser-eject-failed", volumeArg(v)), err)
		return
	}
	i := slices.IndexFunc(vols, func(cur udisks.Volume) bool { return cur.Object == v.Object })
	if i < 0 {
		return
	}
	call, cancel := context.WithTimeout(m.life, mountTimeout)
	defer cancel()
	err = m.Drives.Eject(call, vols[i])
	switch {
	case err == nil:
	case errors.Is(err, udisks.ErrBusy):
		m.notify(i18n.T("file-chooser-eject-failed", volumeArg(v)), i18n.T("file-chooser-eject-busy", volumeArg(v)))
	default:
		m.fail(i18n.T("file-chooser-eject-failed", volumeArg(v)), err)
	}
}

// fail logs err and tells the user summary.
func (m *mounter) fail(summary string, err error) {
	log.Printf("automount: %s: %v", summary, err)
	m.notify(summary, err.Error())
}

func (m *mounter) notify(summary, body string) {
	call, cancel := context.WithTimeout(m.life, callTimeout)
	defer cancel()
	if _, err := m.Notifier.Send(call, i18n.T("volume-notification-source"), summary, body, Icon); err != nil {
		log.Printf("automount: notify %q: %v", summary, err)
	}
}

func volumeArg(v udisks.Volume) i18n.Arg { return i18n.Str("volume", v.Label) }
