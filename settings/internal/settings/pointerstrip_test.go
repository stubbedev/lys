package settings

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/wmipc"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func pointerTestPage() pageSpec {
	return wmPage("wm-test", "ld-mouse-symbolic",
		sectionSpec{title: "wm-section-pointer", hardware: wmipc.ClassPointer, rows: []rowSpec{wmField("input.pointer.accel_profile")}},
		sectionSpec{title: "wm-section-touchpad", hardware: wmipc.ClassTouchpad, rows: []rowSpec{wmField("input.touchpad.tap")}},
		sectionSpec{title: "wm-section-gaps", rows: []rowSpec{wmField("gaps.inner")}},
	)
}

func shownSections(p *settingsPage) map[string]bool {
	shown := map[string]bool{}
	for _, s := range p.sections {
		shown[string(s.hardware)] = s.box.Visible()
	}
	return shown
}

func TestTheMouseAndTouchpadPageGatesEachSectionOnItsDevice(t *testing.T) {
	want := map[string]wmipc.DeviceClass{
		"wm-section-pointer": wmipc.ClassPointer, "wm-section-touchpad": wmipc.ClassTouchpad,
		"wm-section-gestures": wmipc.ClassTouchpad, "wm-section-trackpoint": wmipc.ClassTrackpoint,
		"wm-section-touch-gestures": "", "wm-section-devices": "",
	}
	got := map[string]wmipc.DeviceClass{}
	for _, s := range wmPointerPage().sections {
		got[s.title] = s.hardware
	}
	if !maps.Equal(got, want) {
		t.Fatalf("the sections follow %v, want %v", got, want)
	}
}

func TestASectionShowsOnlyWhileItsDeviceIsConnected(t *testing.T) {
	k, _, fake, _ := wmKit(t, "")
	fake.running = map[string]string{"devices": `[
		{"name":"AT keyboard","capabilities":["keyboard"],"class":"none"},
		{"name":"Synaptics TM3276","capabilities":["pointer"],"class":"touchpad"}]`}
	p := buildPage(k, pointerTestPage())
	if got, want := shownSections(p), map[string]bool{"pointer": false, "touchpad": true}; !maps.Equal(got, want) {
		t.Errorf("with a touchpad and no mouse, the sections show %v, want %v", got, want)
	}
	if text := p.pointers.Text(); !strings.Contains(text, "Synaptics TM3276") || strings.Contains(text, "AT keyboard") || !p.pointers.Visible() {
		t.Errorf("the strip says %q, want the touchpad and not the keyboard", text)
	}
	if !slices.ContainsFunc(p.rows, func(r *settingRow) bool { return r.path == "gaps.inner" }) {
		t.Error("the ungated section lost its row")
	}
}

func TestNoPointingDeviceHidesEverySectionForOne(t *testing.T) {
	k, _, fake, _ := wmKit(t, "")
	fake.running = map[string]string{"devices": `[{"name":"AT keyboard","capabilities":["keyboard"],"class":"none"}]`}
	p := buildPage(k, pointerTestPage())
	if got, want := shownSections(p), map[string]bool{"pointer": false, "touchpad": false}; !maps.Equal(got, want) {
		t.Errorf("with no pointing device, the sections show %v, want %v", got, want)
	}
	if want := locale.T().Get("wm-pointers-none"); p.pointers.Text() != want {
		t.Errorf("the strip says %q, want %q", p.pointers.Text(), want)
	}
}

func TestASettingLinkKeepsItsSectionShownWhenTheDevicesArriveAfterIt(t *testing.T) {
	k, _, fake, _ := wmKit(t, "")
	fake.running = map[string]string{"devices": `[]`}
	var later []func()
	k.background = func(f func()) { later = append(later, f) }
	k.invoke = func(f func()) { f() }
	p := buildPage(k, pointerTestPage())
	touchpad := slices.IndexFunc(p.targets, func(r *settingRow) bool { return r != nil && r.path == "input.touchpad.tap" })
	if p.reveal(touchpad, nil) == nil {
		t.Fatal("the touchpad row cannot be revealed")
	}
	for _, f := range later {
		f()
	}
	if got, want := shownSections(p), map[string]bool{"pointer": false, "touchpad": true}; !maps.Equal(got, want) {
		t.Errorf("with no device connected, after a link to a touchpad setting the sections show %v, want %v", got, want)
	}
	p.showSections()
	if !shownSections(p)["touchpad"] {
		t.Error("the linked section hid again on the next change")
	}
}

func TestEverySectionShowsWhenLysWMCannotSayWhichDevicesAreConnected(t *testing.T) {
	for _, c := range []struct {
		name    string
		running map[string]string
		want    string
	}{
		{"no lys-wm", nil, locale.T().Get("wm-pointers-not-running")},
		{"an older lys-wm", map[string]string{"devices": `[{"name":"Synaptics TM3276","capabilities":["pointer"]}]`}, locale.T().Get("wm-pointers-outdated")},
		{"an answer that is no list", map[string]string{"devices": `{}`}, "could not be read"},
	} {
		k, _, fake, _ := wmKit(t, "")
		fake.running = c.running
		p := buildPage(k, pointerTestPage())
		if got, want := shownSections(p), map[string]bool{"pointer": true, "touchpad": true}; !maps.Equal(got, want) {
			t.Errorf("%s: the sections show %v, want %v", c.name, got, want)
		}
		if text := p.pointers.Text(); !strings.Contains(text, c.want) {
			t.Errorf("%s: the strip says %q, want %q", c.name, text, c.want)
		}
	}
}

func TestOnlyALysWMPageFollowsDevices(t *testing.T) {
	k := testKit(t, "")
	defer func() {
		if recover() == nil {
			t.Fatal("a shell page with a section following lys-wm's devices was built")
		}
	}()
	buildPage(k, pageSpec{id: "shell-test", header: "settings-page-wm-pointer", sections: []sectionSpec{{title: "wm-section-pointer", hardware: wmipc.ClassPointer, rows: fields("bar.location")}}})
}
