package settings

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
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
	return s.hands.offers(finger) && s.enrol.Visible()
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
	s.hands.pick("left-thumb")
	r.l.until("EnrollStart", func() bool { return slices.Contains(r.methods(), "EnrollStart") })
	if !s.progress.Visible() || s.hands.enrolling != "left-thumb" || offered(s, "left-index-finger") || s.deleteAll.Visible() {
		t.Fatal("while enrolling the hands do not show the thumb's progress alone")
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
	if s.hands.enrolling != "" || s.cancel.Visible() || offered(s, "left-thumb") {
		t.Fatal("the ring or cancel outlived the enrolment, or the finger is still offered")
	}
	if s.hint != "The finger is enrolled" {
		t.Fatalf("the hint reads %q after the enrolment", s.hint)
	}
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "EnrollStart", "EnrollStop", "Release"}) {
		t.Fatalf("calls %v", got)
	}
}

func TestCancellingStopsThenReleases(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	s.hands.pick("left-index-finger")
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
			s.hands.pick("left-index-finger")
			r.l.until("EnrollStart", func() bool { return slices.Contains(r.methods(), "EnrollStart") })
			r.emit(t, result, true)
			r.l.until("released", func() bool { return slices.Contains(r.methods(), "Release") })
			if !strings.Contains(s.hint, want) || !s.progress.Visible() {
				t.Fatalf("the hint reads %q (shown %v), want %q in it", s.hint, s.progress.Visible(), want)
			}
			if got := r.methods(); !slices.Equal(got, []string{"Claim", "EnrollStart", "EnrollStop", "Release"}) {
				t.Fatalf("calls %v", got)
			}
		})
	}
}

// unavailableStates are the shown states under p saying a service
// cannot be reached.
func unavailableStates(p widget.Widget) []*cardState {
	var out []*cardState
	for _, s := range treetest.All[*cardState](p) {
		if s.kind == stateUnavailable {
			out = append(out, s)
		}
	}
	return out
}

func unavailableFingerprint(t *testing.T, p widget.Widget) (*fingerHands, *cardState) {
	t.Helper()
	var states []*cardState
	for _, s := range unavailableStates(p) {
		if s.glyph.Name() == "ld-fingerprint-symbolic" {
			states = append(states, s)
		}
	}
	hands := treetest.All[*fingerHands](p)
	if len(states) != 1 || len(hands) != 1 {
		t.Fatalf("%d states and %d hands, want one of each", len(states), len(hands))
	}
	return hands[0], states[0]
}

func TestWithoutAReaderTheHandsAreGreyedOutOverWhyAndNothingIsClaimed(t *testing.T) {
	r := startFprint(t, false)
	r.l.until("the reader asked for", func() bool { c, _ := r.manager.snapshot(); return len(c) == 1 })
	r.l.until("the fingerprint section", func() bool { return len(treetest.All[*fingerHands](r.p)) == 1 })
	if r.p.fingerprint != nil || len(r.methods()) != 0 {
		t.Fatalf("section %v, device calls %v", r.p.fingerprint != nil, r.methods())
	}
	hands, state := unavailableFingerprint(t, r.p)
	if widget.IsEnabled(hands) || slices.ContainsFunc(fingers, hands.offers) {
		t.Error("the hands of a machine without a reader take a finger")
	}
	if want := locale.T().Get("settings-fingerprint-no-reader"); state.Title() != want || state.reveal.button.Visible() {
		t.Errorf("the section says %q, a detail offered %v; want %q and none", state.Title(), state.reveal.button.Visible(), want)
	}
}

func TestAFingerprintServiceThatCannotBeReachedSaysWhy(t *testing.T) {
	p := usersFooter(testKit(t, "")).(*usersPanel)
	t.Cleanup(p.close)
	_, state := unavailableFingerprint(t, p)
	if state.Title() != locale.T().Get("settings-fingerprint-unavailable") || !strings.Contains(state.detail.Text(), "no system bus") {
		t.Errorf("without a system bus the section says %q over %q, want the bus error behind its summary", state.Title(), state.detail.Text())
	}
}

// Deleting one finger asks first: cancelling leaves it enrolled and
// reaches nothing, confirming deletes that finger alone.
func TestAnEnrolledFingerIsDeletedAloneOnceConfirmed(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	pickers := &fakePickers{}
	s.k.pickers = pickers
	treetest.Button(t, s, deleteLabel("right-index-finger")).OnClick()
	if len(pickers.dialogs) != 1 || pickers.dialogs[0].Title != locale.T().Get("settings-fingerprint-delete-finger-title", i18n.Str("finger", fingerName("right-index-finger"))) {
		t.Fatalf("Delete asked %+v", pickers.dialogs)
	}
	pickers.respond(responseCancel)
	r.l.drain()
	if len(r.methods()) != 0 || len(enrolledFingers(s)) != 1 {
		t.Fatalf("cancelling reached the reader %v, %d enrolled", r.methods(), len(enrolledFingers(s)))
	}
	pickers.respond(responseConfirm)
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
	pickers := &fakePickers{}
	s.k.pickers = pickers
	treetest.Button(t, s, deleteLabel("right-index-finger")).OnClick()
	pickers.respond(responseConfirm)
	r.l.until("the refusal reported", func() bool { return r.p.problem.Visible() && slices.Contains(r.methods(), "Release") })
	if got := r.methods(); !slices.Equal(got, []string{"Claim", "DeleteEnrolledFinger", "Release"}) {
		t.Fatalf("calls %v, want no DeleteEnrolledFingers2", got)
	}
	if r.p.problem.Text() != locale.T().Get("settings-fingerprint-reader-failed") || !strings.Contains(r.p.problem.detail.Text(), "Device.DeleteEnrolledFinger: No such method") {
		t.Fatalf("the problem reads %q over %q", r.p.problem.Text(), r.p.problem.detail.Text())
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

func TestTheHandsShowEachFingerWhereItIsAndTheProgressOnTheOneEnrolled(t *testing.T) {
	k := testKit(t, "")
	var picked []string
	h := newFingerHands(k, func(f string) { picked = append(picked, f) })
	h.Arrange(render.Rect{W: handsWidth, H: handsHeight})
	thumb, little := h.handles[slices.Index(fingers, "left-thumb")].Bounds(), h.handles[slices.Index(fingers, "left-little-finger")].Bounds()
	rightThumb := h.handles[slices.Index(fingers, "right-thumb")].Bounds()
	if little.X >= thumb.X || thumb.X >= rightThumb.X {
		t.Errorf("left little %v, left thumb %v, right thumb %v: want them left to right", little, thumb, rightThumb)
	}
	h.set([]string{"right-index-finger"}, "", 0, false)
	h.PressAt(widget.Point{X: thumb.X + thumb.W/2, Y: thumb.Y + thumb.H/2})
	h.handles[slices.Index(fingers, "right-index-finger")].KeyAction(widget.KeyEnter, 0)
	if !slices.Equal(picked, []string{"left-thumb"}) {
		t.Errorf("picked %v, want the left thumb and not the enrolled right index", picked)
	}
	h.set(nil, "left-thumb", 0.4, true)
	h.handles[0].KeyAction(widget.KeyEnter, 0)
	if len(picked) != 1 || h.offers("left-little-finger") {
		t.Error("a finger was offered while another one is enrolled")
	}
	for i, fh := range h.handles {
		if fh.AccessibleName() != fingerName(fingers[i]) {
			t.Errorf("finger %d is named %q", i, fh.AccessibleName())
		}
	}
}

func TestTheFingersEnrolledAreCountedOnTheDefaultReader(t *testing.T) {
	for _, tc := range []struct {
		device   bool
		enrolled []string
		want     int
		noReader bool
	}{
		{true, []string{"right-index-finger", "left-thumb"}, 2, false},
		{true, nil, 0, false},
		{false, nil, 0, true},
	} {
		bus := dbustest.Start(t)
		enrolled := tc.enrolled
		serveFprint(t, bus, tc.device, &enrolled, &atomic.Bool{})
		k := testKit(t, "")
		k.systemBus = func() (*dbus.Conn, error) { return dbus.Connect(bus.Address) }
		n, err := fingersEnrolled(k)
		if n != tc.want || errors.Is(err, errNoReader) != tc.noReader || (err != nil && !tc.noReader) {
			t.Errorf("device %v, enrolled %q: %d fingers, error %v", tc.device, tc.enrolled, n, err)
		}
	}
}

func TestAFingerprintSwitchIsHeldBackUntilAFingerCanUnlock(t *testing.T) {
	tr := locale.T()
	for _, tc := range []struct {
		name    string
		reading fingerprintReading
		on      bool
		status  string
		enrol   bool
		enabled bool
	}{
		{"a finger enrolled", fingerprintReading{fingers: 1}, false, "", false, true},
		{"none enrolled", fingerprintReading{}, false, tr.Get("settings-fingerprint-none"), true, false},
		{"no reader", fingerprintReading{err: errNoReader}, false, tr.Get("settings-fingerprint-no-reader"), false, false},
		{"no reader, but on", fingerprintReading{err: errNoReader}, true, tr.Get("settings-fingerprint-no-reader"), false, true},
		{"fprintd unreachable", fingerprintReading{err: errors.New("the system bus is gone")}, false, tr.Get("settings-fingerprint-unavailable"), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			yaml := "login:\n  lock:\n    fingerprint: false\n"
			if tc.on {
				yaml = "login:\n  lock:\n    fingerprint: true\n"
			}
			k := testKit(t, yaml)
			k.nav = layout(k.store.svc.Config(), bothParts)
			var opened []string
			k.openPage = func(id string) { opened = append(opened, id) }
			was := fingersEnrolled
			fingersEnrolled = func(*kit) (int, error) { return tc.reading.fingers, tc.reading.err }
			t.Cleanup(func() { fingersEnrolled = was })
			c, ok := fieldControl(k, k.store, pageRow(t, "lock", "login.lock.fingerprint")).(*fingerprintSwitch)
			if !ok {
				t.Fatal("the lock's fingerprint row is a plain switch")
			}
			status := ""
			if c.status.Visible() {
				status = c.status.Text()
			}
			if status != tc.status {
				t.Errorf("the status reads %q, want %q", status, tc.status)
			}
			if want := errText(tc.reading.err); !errors.Is(tc.reading.err, errNoReader) && (c.status.detail.Text() != want || c.status.detail.Visible() || c.status.reveal.button.Visible() != (want != "")) {
				t.Errorf("the status explains %q, shown %v, behind a disclosure %v, want %q", c.status.detail.Text(), c.status.detail.Visible(), c.status.reveal.button.Visible(), want)
			}
			if c.status.TooltipText() != "" {
				t.Errorf("the status carries the tooltip %q", c.status.TooltipText())
			}
			if widget.IsEnabled(c.toggle) != tc.enabled {
				t.Errorf("the switch is enabled %v, want %v", widget.IsEnabled(c.toggle), tc.enabled)
			}
			if c.enrol.Visible() != tc.enrol {
				t.Fatalf("the way to enrol a finger is shown %v, want %v", c.enrol.Visible(), tc.enrol)
			}
			if tc.enrol {
				c.enrol.OnClick()
				if !slices.Equal(opened, []string{usersID}) {
					t.Errorf("enrolling opened %q", opened)
				}
			}
		})
	}
	k := testKit(t, "")
	if _, ok := fieldControl(k, k.store, pageRow(t, "lock", "login.greeter.fingerprint")).(*fingerprintSwitch); !ok {
		t.Error("the login screen's fingerprint row is a plain switch")
	}
}

func TestWithoutAWindowDeletingAFingerIsRefused(t *testing.T) {
	r := startFprint(t, true)
	s := r.section(t)
	s.k.pickers = nil
	treetest.Button(t, s, deleteLabel("right-index-finger")).OnClick()
	r.l.drain()
	if !r.p.problem.Visible() || len(r.methods()) != 0 || len(enrolledFingers(s)) != 1 {
		t.Fatalf("problem shown %v, calls %v, %d enrolled", r.p.problem.Visible(), r.methods(), len(enrolledFingers(s)))
	}
}
