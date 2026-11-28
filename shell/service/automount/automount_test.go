package automount_test

import (
	"context"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
	"github.com/stubbedev/lys/shell/service/automount"
	"github.com/stubbedev/lys/shell/service/notifications"
	"github.com/stubbedev/lys/shell/service/udisks"
	"github.com/stubbedev/lys/shell/service/udisks/udiskstest"
)

const (
	stick     = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/stick")
	card      = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/card")
	disk      = dbus.ObjectPath("/org/freedesktop/UDisks2/drives/disk")
	stickPart = dbus.ObjectPath("/org/freedesktop/UDisks2/block_devices/sdb1")
	cardPart  = dbus.ObjectPath("/org/freedesktop/UDisks2/block_devices/mmcblk0p1")
	vault     = dbus.ObjectPath("/org/freedesktop/UDisks2/block_devices/sdc1")
	data      = dbus.ObjectPath("/org/freedesktop/UDisks2/block_devices/sda3")
	root      = dbus.ObjectPath("/org/freedesktop/UDisks2/block_devices/sda2")
)

type question struct {
	summary, body string
	actions       []desktopnotify.Action
}

type notice struct{ summary, body string }

// notifier stands in for the notification daemon: each Ask is handed
// to the test, which answers it with a key, "" closing it unanswered.
type notifier struct {
	asked   chan question
	answers chan string
	sent    chan notice
}

func newNotifier() *notifier {
	return &notifier{asked: make(chan question, 16), answers: make(chan string, 16), sent: make(chan notice, 16)}
}

func (n *notifier) Send(_ context.Context, _, summary, body, _ string) (uint32, error) {
	n.sent <- notice{summary, body}
	return 1, nil
}

func (n *notifier) Ask(ctx context.Context, _, summary, body, _ string, actions []desktopnotify.Action, _ time.Duration) (string, bool, error) {
	n.asked <- question{summary, body, actions}
	select {
	case key := <-n.answers:
		return key, key != "", nil
	case <-ctx.Done():
		return "", false, ctx.Err()
	}
}

type harness struct {
	u       *udiskstest.UDisks
	client  *udisks.Client
	notes   *notifier
	enabled atomic.Bool
	// decided ticks each time the mounter has read Enabled, after the
	// read, so a test that changes the setting once it ticks cannot race
	// the read it waited for.
	decided chan struct{}
	opened  chan string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	u := udiskstest.Start(t)
	h := &harness{u: u, client: udisks.New(u.Client(t)), notes: newNotifier(), decided: make(chan struct{}, 16), opened: make(chan string, 4)}
	h.enabled.Store(true)
	u.AddDrive(t, stick, udiskstest.Drive{Removable: true, CanPowerOff: true})
	u.AddDrive(t, card, udiskstest.Drive{Removable: true, Ejectable: true})
	u.AddDrive(t, disk, udiskstest.Drive{})
	return h
}

func (h *harness) start(t *testing.T) {
	t.Helper()
	stop, err := automount.Start(automount.Deps{
		Drives:   udisks.New(h.u.Client(t)),
		Notifier: h.notes,
		Enabled: func() bool {
			on := h.enabled.Load()
			h.decided <- struct{}{}
			return on
		},
		Open: func(dir string) { h.opened <- dir },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
}

func (h *harness) mountPoint(t *testing.T, object dbus.ObjectPath) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	vols, err := h.client.Volumes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(vols, func(v udisks.Volume) bool { return v.Object == object })
	if i < 0 {
		t.Fatalf("no volume %s", object)
	}
	return vols[i].MountPoints
}

func (h *harness) mounts() []string {
	var out []string
	for _, c := range h.u.Calls.Load() {
		if strings.HasPrefix(c, udiskstest.CallMount+" ") || strings.HasPrefix(c, udiskstest.CallUnlock+" ") {
			out = append(out, c)
		}
	}
	return out
}

func mounted(object dbus.ObjectPath) string { return udiskstest.CallMount + " " + string(object) }

func summaryFor(label string) string { return i18n.T("volume-mounted", i18n.Str("volume", label)) }

// A filesystem on a removable drive plugged in is mounted, and the
// notification names it, says where, and offers Open and Eject.
func TestARemovableFilesystemPluggedInIsMountedAndAnnounced(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	waittest.For(t, h.decided, "the startup pass")
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	q := waittest.For(t, h.notes.asked, "the stick's notification")
	at := h.mountPoint(t, stickPart)
	if len(at) != 1 || !strings.HasPrefix(at[0], h.u.MountRoot) {
		t.Fatalf("the stick is mounted at %v, want once under %s", at, h.u.MountRoot)
	}
	if q.summary != summaryFor("STICK") || q.body != at[0] {
		t.Errorf("the notification reads %q / %q, want %q / %q", q.summary, q.body, summaryFor("STICK"), at[0])
	}
	want := []desktopnotify.Action{
		{Key: notifications.DefaultActionID, Label: i18n.T("volume-action-open")},
		{Key: automount.ActionEject, Label: i18n.T("volume-action-eject")},
	}
	if !slices.Equal(q.actions, want) {
		t.Errorf("the notification offers %+v, want %+v", q.actions, want)
	}
	if got := h.mounts(); !slices.Equal(got, []string{mounted(stickPart)}) {
		t.Errorf("mounted %v, want the stick once", got)
	}
}

// A filesystem on an internal drive, system or not, is never mounted:
// the stick plugged in after it is the only mount.
func TestAnInternalFilesystemIsLeftAlone(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	waittest.For(t, h.decided, "the startup pass")
	h.u.AddVolume(t, data, udiskstest.Volume{Label: "data", Drive: disk})
	h.u.AddVolume(t, root, udiskstest.Volume{Label: "root", Drive: disk, HintSystem: true})
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	if q := waittest.For(t, h.notes.asked, "the stick's notification"); q.summary != summaryFor("STICK") {
		t.Errorf("the first notification is %q, want the stick's", q.summary)
	}
	if got := h.mounts(); !slices.Equal(got, []string{mounted(stickPart)}) {
		t.Errorf("mounted %v, want the stick alone", got)
	}
	if at := h.mountPoint(t, data); len(at) != 0 {
		t.Errorf("the internal data partition is mounted at %v", at)
	}
}

// An encrypted container plugged in stays locked, and what the file
// manager unlocks is left for it to mount.
func TestAnEncryptedVolumeIsLeftLocked(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	waittest.For(t, h.decided, "the startup pass")
	h.u.AddVolume(t, vault, udiskstest.Volume{Label: "VAULT", Drive: stick, Encrypted: true})
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	waittest.For(t, h.notes.asked, "the stick's notification")
	if _, err := h.client.Unlock(context.Background(), vault, []byte(h.u.Passphrase)); err != nil {
		t.Fatal(err)
	}
	h.u.AddVolume(t, cardPart, udiskstest.Volume{Label: "CARD", Drive: card})
	if q := waittest.For(t, h.notes.asked, "the card's notification"); q.summary != summaryFor("CARD") {
		t.Errorf("the notification after the unlock is %q, want the card's", q.summary)
	}
	want := []string{mounted(stickPart), udiskstest.CallUnlock + " " + string(vault), mounted(cardPart)}
	if got := h.mounts(); !slices.Equal(got, want) {
		t.Errorf("calls %v, want %v: the vault neither unlocked nor its cleartext mounted by the mounter", got, want)
	}
}

// The removable volumes present at start are mounted; one already
// mounted is not mounted again, nor an internal one.
func TestTheStartupPassMountsWhatIsPluggedIn(t *testing.T) {
	h := newHarness(t)
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	h.u.AddVolume(t, cardPart, udiskstest.Volume{Label: "CARD", Drive: card})
	h.u.AddVolume(t, data, udiskstest.Volume{Label: "data", Drive: disk})
	if _, err := h.client.Mount(context.Background(), cardPart); err != nil {
		t.Fatal(err)
	}
	h.u.Calls.Store(nil)
	h.start(t)
	if q := waittest.For(t, h.notes.asked, "the stick's notification"); q.summary != summaryFor("STICK") {
		t.Errorf("the startup notification is %q, want the stick's", q.summary)
	}
	h.u.AddVolume(t, vault, udiskstest.Volume{Drive: stick, Encrypted: true})
	h.u.AddVolume(t, stickPart+"2", udiskstest.Volume{Label: "MORE", Drive: stick})
	waittest.For(t, h.notes.asked, "the second partition's notification")
	if got := h.mounts(); !slices.Equal(got, []string{mounted(stickPart), mounted(stickPart + "2")}) {
		t.Errorf("mounted %v, want the stick at start and the partition plugged in after", got)
	}
}

// With the setting off nothing is mounted, at start or plugged in; a
// reload turning it on applies to the next drive.
func TestTheSettingTurnsItOff(t *testing.T) {
	h := newHarness(t)
	h.enabled.Store(false)
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	h.start(t)
	waittest.For(t, h.decided, "the startup pass")
	h.u.AddVolume(t, cardPart, udiskstest.Volume{Label: "CARD", Drive: card})
	waittest.For(t, h.decided, "the card plugged in")
	if got := h.mounts(); len(got) != 0 {
		t.Errorf("mounted %v with the setting off", got)
	}
	select {
	case q := <-h.notes.asked:
		t.Errorf("announced %q with the setting off", q.summary)
	default:
	}
	h.enabled.Store(true)
	h.u.AddVolume(t, stickPart+"2", udiskstest.Volume{Label: "MORE", Drive: stick})
	if q := waittest.For(t, h.notes.asked, "the partition plugged in once on"); q.summary != summaryFor("MORE") {
		t.Errorf("the notification once on is %q, want the partition's", q.summary)
	}
	if got := h.mounts(); !slices.Equal(got, []string{mounted(stickPart + "2")}) {
		t.Errorf("mounted %v once on, want only what was plugged in after", got)
	}
}

// The notification's body opens the mount point; closing it does
// nothing.
func TestTheNotificationOpensTheVolume(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	q := waittest.For(t, h.notes.asked, "the stick's notification")
	h.notes.answers <- notifications.DefaultActionID
	if dir := waittest.For(t, h.opened, "the stick opened"); dir != q.body {
		t.Errorf("opened %q, want the mount point %q", dir, q.body)
	}
	h.u.AddVolume(t, cardPart, udiskstest.Volume{Label: "CARD", Drive: card})
	waittest.For(t, h.notes.asked, "the card's notification")
	h.notes.answers <- ""
	h.u.AddVolume(t, stickPart+"2", udiskstest.Volume{Label: "MORE", Drive: stick})
	waittest.For(t, h.notes.asked, "the partition's notification")
	select {
	case dir := <-h.opened:
		t.Errorf("a notification closed unanswered opened %q", dir)
	default:
	}
	if slices.ContainsFunc(h.u.Calls.Load(), func(c string) bool { return strings.HasPrefix(c, udiskstest.CallUnmount) }) {
		t.Error("a notification answered with Open or closed unmounted a volume")
	}
}

// Eject unmounts the volume and powers its drive off; a volume in use
// stays mounted and says so.
func TestTheNotificationEjectsTheVolume(t *testing.T) {
	h := newHarness(t)
	h.start(t)
	h.u.AddVolume(t, stickPart, udiskstest.Volume{Label: "STICK", Drive: stick})
	waittest.For(t, h.notes.asked, "the stick's notification")
	h.u.Busy.Store(true)
	h.notes.answers <- automount.ActionEject
	n := waittest.For(t, h.notes.sent, "the busy volume's notice")
	label := i18n.Str("volume", "STICK")
	if n.summary != i18n.T("file-chooser-eject-failed", label) || n.body != i18n.T("file-chooser-eject-busy", label) {
		t.Errorf("a busy eject said %+v", n)
	}
	if calls := h.u.Calls.Load(); slices.Contains(calls, udiskstest.CallPowerOff+" "+string(stick)) {
		t.Errorf("a busy volume's drive was powered off: %v", calls)
	}
	h.u.Busy.Store(false)
	h.u.AddVolume(t, cardPart, udiskstest.Volume{Label: "CARD", Drive: card})
	waittest.For(t, h.notes.asked, "the card's notification")
	h.u.Calls.Store(nil)
	h.notes.answers <- automount.ActionEject
	want := []string{udiskstest.CallUnmount + " " + string(cardPart), udiskstest.CallEject + " " + string(card)}
	waittest.Until(t, func() bool { return slices.Equal(h.u.Calls.Load(), want) }, "the card unmounted and ejected")
	if at := h.mountPoint(t, cardPart); len(at) != 0 {
		t.Errorf("the ejected card is still mounted at %v", at)
	}
}
