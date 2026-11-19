package settings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	fprintName      = "net.reactivated.Fprint"
	fprintManager   = fprintName + ".Manager"
	fprintDevice    = fprintName + ".Device"
	fprintPath      = dbus.ObjectPath("/net/reactivated/Fprint/Manager")
	errNoEnrolled   = fprintName + ".Error.NoEnrolledPrints"
	errPermission   = fprintName + ".Error.PermissionDenied"
	statusStage     = "enroll-stage-passed"
	statusCompleted = "enroll-completed"
	statusDuplicate = "enroll-duplicate"
	statusDataFull  = "enroll-data-full"
	statusFailed    = "enroll-failed"
	statusDisconn   = "enroll-disconnected"
	statusUnknown   = "enroll-unknown-error"
)

var fingers = []string{
	"left-little-finger", "left-ring-finger", "left-middle-finger", "left-index-finger", "left-thumb",
	"right-thumb", "right-index-finger", "right-middle-finger", "right-ring-finger", "right-little-finger",
}

func endsEnrolment(status string) bool {
	return slices.Contains([]string{statusCompleted, statusDuplicate, statusDataFull, statusFailed, statusDisconn, statusUnknown}, status)
}

type fprint struct {
	conn   *dbus.Conn
	device sysService
	user   string
}

func openFprint(ctx context.Context, conn *dbus.Conn, user string) (*fprint, error) {
	var path dbus.ObjectPath
	if err := conn.Object(fprintName, fprintPath).CallWithContext(ctx, fprintManager+".GetDefaultDevice", 0).Store(&path); err != nil {
		return nil, fmt.Errorf("find a fingerprint reader: %w", err)
	}
	return &fprint{conn: conn, device: sysService{conn: conn, name: fprintName, iface: fprintDevice, path: path}, user: user}, nil
}

func (f *fprint) enrolled(ctx context.Context) ([]string, error) {
	var out []string
	err := f.device.store(ctx, "ListEnrolledFingers", &out, f.user)
	var derr dbus.Error
	if errors.As(err, &derr) && derr.Name == errNoEnrolled {
		return nil, nil
	}
	return out, err
}

func (f *fprint) stages(ctx context.Context) (int, error) {
	props, err := f.device.props(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := props["num-enroll-stages"].Value().(int32)
	if n <= 0 {
		return 0, fmt.Errorf("the reader reports %d enrolment stages", n)
	}
	return int(n), nil
}

type enrolment struct {
	Result string `dbus:"result"`
	Done   bool   `dbus:"done"`
}

var enrollStatus = dbusx.NewSignal[enrolment](fprintDevice, "EnrollStatus")

func (f *fprint) watch(status func(result string, done bool), failed func(error)) (*dbusx.Watch, error) {
	w, err := dbusx.WatchSignals(context.Background(), f.conn, dbusx.FromOwner(fprintName), failed,
		dbusx.On(enrollStatus, f.device.path, func(e enrolment, _ dbus.Sequence) error {
			status(e.Result, e.Done)
			return nil
		}))
	if err != nil {
		return nil, fmt.Errorf("follow the reader's enrolment: %w", err)
	}
	return w, nil
}

type enrollRing struct {
	finger                  string
	stages, passed, updates int
}

func (r *enrollRing) set(passed int) {
	r.passed = passed
	r.updates++
}

func (r enrollRing) fraction() float64 { return float64(r.passed) / float64(max(1, r.stages)) }

type fingerprintSection struct {
	*widget.ItemGroup
	k         *kit
	fp        *fprint
	fail      func(error)
	enrolled  []string
	enrol     *widget.Box
	hands     *fingerHands
	progress  *widget.Box
	ring      *enrollRing
	hint      *widget.Label
	cancel    *widget.Button
	deleteAll *widget.Box
	note      *widget.Box
	watch     *dbusx.Watch
	busy      bool
}

func (p *usersPanel) loadFingerprint() {
	if p.conn == nil || p.user == "" {
		return
	}
	conn, user := p.conn, p.user
	type opened struct {
		fp       *fprint
		stages   int
		enrolled []string
		err      error
	}
	async(p.k, func() opened {
		return withTimeoutValue(func(ctx context.Context) opened {
			fp, err := openFprint(ctx, conn, user)
			if err != nil {
				return opened{err: err}
			}
			stages, err := fp.stages(ctx)
			if err != nil {
				return opened{err: err}
			}
			enrolled, err := fp.enrolled(ctx)
			return opened{fp, stages, enrolled, err}
		})
	}, func(o opened) {
		if o.err != nil {
			return
		}
		s := newFingerprintSection(p.k, o.fp, o.stages, o.enrolled, p.fail)
		p.fingerprint = s
		p.Append(newSection(p.k, "settings-section-users-fingerprint", s), false)
	})
}

func withTimeoutValue[T any](fn func(context.Context) T) T {
	v, _ := withTimeout(func(ctx context.Context) (T, error) { return fn(ctx), nil })
	return v
}

func newFingerprintSection(k *kit, fp *fprint, stages int, enrolled []string, fail func(error)) *fingerprintSection {
	t := locale.T()
	s := &fingerprintSection{ItemGroup: widget.NewItemGroup(), k: k, fp: fp, fail: fail, enrolled: enrolled}
	s.hands = newFingerHands(k, s.enroll)
	s.hands.SetAccessibleName(t.Get("settings-fingerprint-progress"))
	s.enrol = itemColumn(
		k.label(t.Get("settings-fingerprint-enrol"), "setting-label"),
		wrapped(k.label(t.Attr("settings-fingerprint-enrol", "description"), "setting-description", css.Caption, css.DimLabel)),
		s.hands,
	)
	s.ring = &enrollRing{stages: stages}
	s.hint = wrapped(k.label("", "setting-description"))
	s.cancel = k.button(k.label(t.Get("settings-fingerprint-cancel")), s.finish, css.Small, css.Secondary)
	s.progress = gapBox(widget.Row)
	s.progress.SetElement("item")
	s.progress.AppendAligned(s.hint, true, widget.AlignCenter)
	s.progress.AppendAligned(s.cancel, false, widget.AlignCenter)
	s.deleteAll = itemRow(k.button(k.label(t.Get("settings-fingerprint-delete-all")), s.confirmDeleteAll, css.Small, css.Destructive))
	s.note = insetRow(wrapped(k.label(t.Get("settings-fingerprint-lock"), "setting-description", css.Caption, css.DimLabel)))
	s.show()
	return s
}

func itemColumn(children ...widget.Widget) *widget.Box {
	col := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	col.SetElement("item")
	for _, c := range children {
		col.Append(c, false)
	}
	return col
}

func itemRow(w widget.Widget) *widget.Box {
	row := gapBox(widget.Row)
	row.SetElement("item")
	row.Append(w, false)
	return row
}

func fingerName(finger string) string {
	return locale.T().Get("settings-fingerprint-" + finger)
}

func (s *fingerprintSection) show() {
	s.Clear()
	enrolling := ""
	if s.busy {
		enrolling = s.ring.finger
	}
	s.hands.set(s.enrolled, enrolling, s.ring.fraction(), s.busy)
	for _, f := range fingers {
		if slices.Contains(s.enrolled, f) {
			s.Append(s.enrolledRow(f))
		}
	}
	if len(s.enrolled) == 0 {
		s.Append(s.k.emptyRow("ld-lock-open-symbolic", locale.T().Get("settings-fingerprint-none")))
	}
	s.enrol.SetVisible(s.busy || len(s.enrolled) < len(fingers))
	s.cancel.SetVisible(s.busy)
	s.progress.SetVisible(s.hint.Text() != "")
	s.deleteAll.SetVisible(!s.busy && len(s.enrolled) > 0)
	for _, w := range []widget.Widget{s.enrol, s.progress, s.deleteAll, s.note} {
		s.Append(w)
	}
}

func (s *fingerprintSection) enrolledRow(finger string) *widget.Item {
	name := fingerName(finger)
	row := widget.NewItem(s.k.face, name, "")
	row.AddClass(css.Small, "fingerprint-enrolled")
	label := deleteLabel(finger)
	del := s.k.button(s.k.icon("ld-trash-2-symbolic"), func() { s.deleteFinger(finger) }, css.Icon, css.Ghost)
	del.SetTooltip(label)
	del.SetAccessibleName(label)
	del.SetEnabled(!s.busy)
	row.AddAction(del)
	return row
}

func deleteLabel(finger string) string {
	return locale.T().Get("settings-fingerprint-delete", i18n.Str("finger", fingerName(finger)))
}

func (s *fingerprintSection) say(text string) {
	s.hint.SetText(text)
	s.show()
}

func (s *fingerprintSection) call(fn func(context.Context) error) {
	async(s.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) { return struct{}{}, fn(ctx) })
		return err
	}, func(err error) {
		s.fail(permissionMessage(err))
		s.reload()
	})
}

func permissionMessage(err error) error {
	var derr dbus.Error
	if errors.As(err, &derr) && derr.Name == errPermission {
		return errors.New(locale.T().Get("settings-fingerprint-denied"))
	}
	return err
}

func (s *fingerprintSection) reload() {
	fp := s.fp
	async(s.k, func() fingersListing {
		v, err := withTimeout(fp.enrolled)
		return fingersListing{v, err}
	}, func(l fingersListing) {
		if l.err != nil {
			s.fail(l.err)
			return
		}
		s.enrolled = l.names
		s.show()
	})
}

type fingersListing struct {
	names []string
	err   error
}

func (s *fingerprintSection) enroll(finger string) {
	if s.busy {
		return
	}
	watch, err := s.fp.watch(func(result string, done bool) {
		s.k.invoke(func() { s.status(result, done) })
	}, func(err error) {
		s.k.invoke(func() { s.fail(err) })
	})
	if err != nil {
		s.fail(err)
		return
	}
	s.watch, s.busy = watch, true
	s.ring.finger, s.ring.passed, s.ring.updates = finger, 0, 0
	s.say(locale.T().Get("settings-fingerprint-place"))
	fp := s.fp
	async(s.k, func() error {
		_, err := withTimeout(func(ctx context.Context) (struct{}, error) {
			if err := fp.device.call(ctx, "Claim", fp.user); err != nil {
				return struct{}{}, err
			}
			if err := fp.device.call(ctx, "EnrollStart", finger); err != nil {
				return struct{}{}, errors.Join(err, fp.device.call(ctx, "Release"))
			}
			return struct{}{}, nil
		})
		return err
	}, func(err error) {
		if err != nil {
			s.fail(errors.Join(permissionMessage(err), s.unwatch()))
			s.say("")
		}
	})
}

func (s *fingerprintSection) status(result string, done bool) {
	if !s.busy {
		return
	}
	t := locale.T()
	switch result {
	case statusStage:
		s.ring.set(s.ring.passed + 1)
	case statusCompleted:
		s.ring.set(s.ring.stages)
	}
	s.hands.set(s.enrolled, s.ring.finger, s.ring.fraction(), s.busy)
	hint := t.Get("settings-fingerprint-place")
	if key := "settings-fingerprint-" + strings.TrimPrefix(result, "enroll-"); t.Has(key) {
		hint = t.Get(key)
	}
	s.hint.SetText(hint)
	if done || endsEnrolment(result) {
		s.finish()
	}
}

func (s *fingerprintSection) unwatch() error {
	var err error
	if s.watch != nil {
		err = s.watch.Stop()
		s.watch = nil
	}
	s.busy = false
	return err
}

func (s *fingerprintSection) finish() {
	if !s.busy {
		return
	}
	if err := s.unwatch(); err != nil {
		s.fail(err)
	}
	s.show()
	fp := s.fp
	s.call(func(ctx context.Context) error {
		return errors.Join(fp.device.call(ctx, "EnrollStop"), fp.device.call(ctx, "Release"))
	})
}

func (s *fingerprintSection) close() {
	if !s.busy {
		return
	}
	unwatched := s.unwatch()
	fp := s.fp
	_, err := withTimeout(func(ctx context.Context) (struct{}, error) {
		return struct{}{}, errors.Join(fp.device.call(ctx, "EnrollStop"), fp.device.call(ctx, "Release"))
	})
	if err := errors.Join(unwatched, err); err != nil {
		log.Printf("settings: end the fingerprint enrolment: %v", err)
	}
}

func (s *fingerprintSection) confirmDeleteAll() {
	if s.k.pickers == nil {
		s.fail(errors.New("settings: delete every fingerprint: no window to ask in"))
		return
	}
	s.k.pickers.dialog(confirmDialog(s.k, "settings-fingerprint-delete-all", s.deleteAllFingers))
}

func (s *fingerprintSection) deleteAllFingers() {
	if s.busy {
		return
	}
	s.say("")
	fp := s.fp
	s.call(func(ctx context.Context) error {
		if err := fp.device.call(ctx, "Claim", fp.user); err != nil {
			return err
		}
		err := fp.device.call(ctx, "DeleteEnrolledFingers2")
		return errors.Join(err, fp.device.call(ctx, "Release"))
	})
}

func (s *fingerprintSection) deleteFinger(finger string) {
	if s.busy {
		return
	}
	s.say("")
	fp := s.fp
	s.call(func(ctx context.Context) error {
		if err := fp.device.call(ctx, "Claim", fp.user); err != nil {
			return err
		}
		err := fp.device.call(ctx, "DeleteEnrolledFinger", finger)
		return errors.Join(err, fp.device.call(ctx, "Release"))
	})
}
