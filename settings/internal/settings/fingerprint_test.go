package settings

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
)

const fakeReader = dbus.ObjectPath("/net/reactivated/Fprint/Device/0")

type fakeFprintManager struct {
	*fakeSys
	device bool
}

func (f fakeFprintManager) GetDefaultDevice() (dbus.ObjectPath, *dbus.Error) {
	f.record("GetDefaultDevice")
	if !f.device {
		return "/", dbus.NewError(fprintName+".Error.NoSuchDevice", []any{"No devices available"})
	}
	return fakeReader, nil
}

type fakeFprintDevice struct {
	*fakeSys
	enrolled         *[]string
	unknownDeleteOne *atomic.Bool
}

func (f fakeFprintDevice) Claim(string) *dbus.Error { return f.record("Claim") }
func (f fakeFprintDevice) Release() *dbus.Error     { return f.record("Release") }
func (f fakeFprintDevice) EnrollStop() *dbus.Error  { return f.record("EnrollStop") }
func (f fakeFprintDevice) EnrollStart(finger string) *dbus.Error {
	return f.record("EnrollStart", finger)
}

func (f fakeFprintDevice) DeleteEnrolledFinger(finger string) *dbus.Error {
	if f.unknownDeleteOne.Load() {
		f.record("DeleteEnrolledFinger", finger)
		return dbus.NewError("org.freedesktop.DBus.Error.UnknownMethod", []any{"No such method DeleteEnrolledFinger"})
	}
	f.mu.Lock()
	*f.enrolled = slices.DeleteFunc(*f.enrolled, func(s string) bool { return s == finger })
	f.mu.Unlock()
	return f.record("DeleteEnrolledFinger", finger)
}

func (f fakeFprintDevice) DeleteEnrolledFingers2() *dbus.Error {
	return f.record("DeleteEnrolledFingers2")
}

func (f fakeFprintDevice) ListEnrolledFingers(string) ([]string, *dbus.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(*f.enrolled) == 0 {
		return nil, dbus.NewError(errNoEnrolled, []any{"No fingers enrolled"})
	}
	return slices.Clone(*f.enrolled), nil
}

type fprintRig struct {
	unknownDeleteOne atomic.Bool
	bus              *dbustest.Bus
	manager          *fakeSys
	device           *fakeSys
	enrolled         []string
	p                *usersPanel
	l                *loop
}

func startFprint(t *testing.T, withDevice bool) *fprintRig {
	t.Helper()
	return startFprintWith(t, withDevice, "right-index-finger")
}

func startFprintWith(t *testing.T, withDevice bool, enrolled ...string) *fprintRig {
	t.Helper()
	r := &fprintRig{bus: dbustest.Start(t), enrolled: enrolled}
	r.manager, r.device = serveFprint(t, r.bus, withDevice, &r.enrolled, &r.unknownDeleteOne)
	t.Setenv("USER", "alice")
	r.p, r.l = usersFixture(t, r.bus)
	return r
}

func serveFprint(t *testing.T, bus *dbustest.Bus, withDevice bool, enrolled *[]string, unknownDeleteOne *atomic.Bool) (manager, device *fakeSys) {
	t.Helper()
	conn := bus.Conn(t)
	manager = &fakeSys{props: map[string]any{}}
	device = &fakeSys{props: map[string]any{"num-enroll-stages": int32(5), "name": "Fake reader", "scan-type": "press"}, conn: conn, path: fakeReader, iface: fprintDevice}
	if _, err := dbusx.Export(conn, fprintPath, dbusx.Interface{Name: fprintManager, Methods: fakeFprintManager{manager, withDevice}}); err != nil {
		t.Fatal(err)
	}
	if _, err := dbusx.Export(conn, fakeReader, dbusx.Interface{Name: fprintDevice, Methods: fakeFprintDevice{device, enrolled, unknownDeleteOne}, Properties: device.getters()}); err != nil {
		t.Fatal(err)
	}
	if _, err := dbusx.Own(conn, fprintName); err != nil {
		t.Fatal(err)
	}
	return manager, device
}

func (r *fprintRig) emit(t *testing.T, result string, done bool) {
	t.Helper()
	if err := enrollStatus.Emit(r.device.conn, fakeReader, enrolment{Result: result, Done: done}); err != nil {
		t.Fatal(err)
	}
}

func (r *fprintRig) methods() []string {
	calls, _ := r.device.snapshot()
	out := make([]string, len(calls))
	for i, c := range calls {
		out[i] = c.method
	}
	return out
}

func (r *fprintRig) section(t *testing.T) *fingerprintSection {
	t.Helper()
	r.l.until("the fingerprint section", func() bool { return r.p.fingerprint != nil })
	return r.p.fingerprint
}

func enrolledFingers(s *fingerprintSection) []string {
	var out []string
	for _, f := range fingers {
		name := deleteLabel(f)
		treetest.Walk(s, func(w widget.Widget) {
			if widget.Describe(w).Name == name {
				out = append(out, f)
			}
		})
	}
	return out
}

func offered(s *fingerprintSection, finger string) bool {
	found := false
	treetest.Walk(s.choices, func(w widget.Widget) {
		found = found || widget.Describe(w).Name == fingerName(finger)
	})
	return found && s.enrol.Visible()
}

func TestTheEnrolledFingerIsListedAndNotOfferedAgain(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	if got := enrolledFingers(s); !slices.Equal(got, []string{"right-index-finger"}) {
		t.Fatalf("listed %v, want the right index", got)
	}
	if offered(s, "right-index-finger") || !offered(s, "left-thumb") {
		t.Fatal("the enrolled finger is offered for enrolment, or another is not")
	}
	if s.progress.Visible() || !s.deleteAll.Visible() {
		t.Fatalf("progress shown %v, delete all shown %v before any enrolment", s.progress.Visible(), s.deleteAll.Visible())
	}
}

func TestWithNoFingerEnrolledThereIsNothingToDelete(t *testing.T) {
	r := startFprintWith(t, true)
	s := r.section(t)
	if len(enrolledFingers(s)) != 0 || s.deleteAll.Visible() {
		t.Fatalf("listed %v, delete all shown %v with nothing enrolled", enrolledFingers(s), s.deleteAll.Visible())
	}
	if !slices.Contains(treetest.Labels(s), "No finger enrolled") {
		t.Fatalf("labels %v, want the empty row", treetest.Labels(s))
	}
}

func TestAFiveStageEnrolmentFillsTheRingFiveTimesAndListsTheFinger(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	treetest.Button(t, s.choices, "Left thumb").OnClick()
	r.l.until("EnrollStart", func() bool { return slices.Contains(r.methods(), "EnrollStart") })
	if !s.progress.Visible() || !s.ring.Visible() || s.enrol.Visible() || s.deleteAll.Visible() {
		t.Fatal("while enrolling the progress is not the only thing offered")
	}
	r.enrolled = append(r.enrolled, "left-thumb")
	for range 4 {
		r.emit(t, statusStage, false)
	}
	r.emit(t, statusCompleted, true)
	r.l.until("the finger listed", func() bool { return slices.Contains(enrolledFingers(s), "left-thumb") })
	if s.ring.updates != 5 || s.ring.passed != 5 {
		t.Fatalf("the ring moved %d times to %d stages for 5 signals", s.ring.updates, s.ring.passed)
	}
	if s.ring.Visible() || s.cancel.Visible() || offered(s, "left-thumb") {
		t.Fatal("the ring or cancel outlived the enrolment, or the finger is still offered")
	}
	if s.hint.Text() != "The finger is enrolled" {
		t.Fatalf("the hint reads %q after the enrolment", s.hint.Text())
	}
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "EnrollStart", "EnrollStop", "Release"}) {
		t.Fatalf("calls %v", got)
	}
}

func TestCancellingStopsThenReleases(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	treetest.Button(t, s.choices, "Left index").OnClick()
	r.l.until("EnrollStart", func() bool { return slices.Contains(r.methods(), "EnrollStart") })
	r.emit(t, statusStage, false)
	r.l.until("one stage", func() bool { return s.ring.passed == 1 })
	treetest.Button(t, s, "Cancel").OnClick()
	r.l.until("released", func() bool { return slices.Contains(r.methods(), "Release") })
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "EnrollStart", "EnrollStop", "Release"}) {
		t.Fatalf("calls %v, want EnrollStop before Release", got)
	}
	r.emit(t, statusStage, false)
	r.l.drain()
	if s.ring.updates != 1 {
		t.Fatalf("the ring moved %d times after a cancel", s.ring.updates)
	}
	if !s.enrol.Visible() {
		t.Fatal("after a cancel no finger is offered")
	}
}

func TestADuplicateOrAFullReaderSaysSoAndReleases(t *testing.T) {
	for result, want := range map[string]string{statusDuplicate: "already enrolled", statusDataFull: "full"} {
		t.Run(result, func(t *testing.T) {
			r := startFprint(t, true)
			s := r.section(t)
			treetest.Button(t, s.choices, "Left index").OnClick()
			r.l.until("EnrollStart", func() bool { return slices.Contains(r.methods(), "EnrollStart") })
			r.emit(t, result, true)
			r.l.until("released", func() bool { return slices.Contains(r.methods(), "Release") })
			if !strings.Contains(s.hint.Text(), want) || !s.progress.Visible() {
				t.Fatalf("the hint reads %q (shown %v), want %q in it", s.hint.Text(), s.progress.Visible(), want)
			}
			if got := r.methods(); !slices.Equal(got, []string{"Claim", "EnrollStart", "EnrollStop", "Release"}) {
				t.Fatalf("calls %v", got)
			}
		})
	}
}

func TestWithoutAReaderThereIsNoSectionAndNoClaim(t *testing.T) {
	r := startFprint(t, false)
	r.l.until("the reader asked for", func() bool { c, _ := r.manager.snapshot(); return len(c) == 1 })
	r.l.drain()
	if r.p.fingerprint != nil || len(r.methods()) != 0 {
		t.Fatalf("section %v, device calls %v", r.p.fingerprint != nil, r.methods())
	}
}

func TestAnEnrolledFingerIsDeletedAlone(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	treetest.Button(t, s, deleteLabel("right-index-finger")).OnClick()
	r.l.until("the finger gone", func() bool {
		return len(enrolledFingers(s)) == 0 && slices.Contains(r.methods(), "Release")
	})
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "DeleteEnrolledFinger", "Release"}) {
		t.Fatalf("calls %v", got)
	}
}

func TestAReaderThatCannotDeleteOneFingerKeepsEveryOther(t *testing.T) {
	r := startFprint(t, true)
	r.unknownDeleteOne.Store(true)
	s := r.section(t)
	treetest.Button(t, s, deleteLabel("right-index-finger")).OnClick()
	r.l.until("the refusal reported", func() bool { return r.p.problem.Visible() && slices.Contains(r.methods(), "Release") })
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "DeleteEnrolledFinger", "Release"}) {
		t.Fatalf("calls %v, want no DeleteEnrolledFingers2", got)
	}
	if !strings.Contains(r.p.problem.Text(), "Device.DeleteEnrolledFinger: No such method") {
		t.Fatalf("the problem reads %q", r.p.problem.Text())
	}
}

func TestDeletingEveryFingerprintAsksFirst(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	pickers := &fakePickers{}
	s.k.pickers = pickers
	treetest.Button(t, s, "Delete All Fingerprints").OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("delete all opened %d dialogs, want one question", len(pickers.dialogs))
	}
	ask := pickers.dialogs[0]
	if ask.Title != "Delete All Fingerprints?" || !ask.Modal || ask.CancelResponse != responseCancel || ask.DefaultResponse != "" {
		t.Fatalf("dialog %q modal %v cancel %q default %q", ask.Title, ask.Modal, ask.CancelResponse, ask.DefaultResponse)
	}
	ask.OnResponse(responseCancel)
	r.l.drain()
	if len(r.methods()) != 0 {
		t.Fatalf("cancelling reached the reader: %v", r.methods())
	}
	ask.OnResponse(responseConfirm)
	r.l.until("released", func() bool { return slices.Contains(r.methods(), "Release") })
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "DeleteEnrolledFingers2", "Release"}) {
		t.Fatalf("calls %v", got)
	}
}

func TestWithoutAWindowDeletingEveryFingerprintIsRefused(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	s.k.pickers = nil
	treetest.Button(t, s, "Delete All Fingerprints").OnClick()
	if !r.p.problem.Visible() || len(r.methods()) != 0 {
		t.Fatalf("problem shown %v, calls %v", r.p.problem.Visible(), r.methods())
	}
}
