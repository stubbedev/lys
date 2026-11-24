package app

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const refusedURI = "https://refused.example"

type openURIIface struct{ d *mockDesktop }

func (o openURIIface) OpenURI(sender dbus.Sender, _, uri string, opts map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	o.d.mu.Lock()
	o.d.opened = append(o.d.opened, uri)
	o.d.mu.Unlock()
	path := o.d.handle(sender, opts)
	code := uint32(0)
	if uri == refusedURI {
		code = 2
	}
	o.d.respond(sender, path, code, map[string]dbus.Variant{})
	return path, nil
}

func launchApp(t *testing.T) *Application {
	t.Helper()
	a := testApp(nil)
	a.launchState = launcher{openURI: a.portalOpenURI}
	t.Cleanup(func() { a.desktop().close() })
	return a
}

func openAndWait(t *testing.T, a *Application, open func(string, func(error)), target string) error {
	t.Helper()
	var result *error
	open(target, func(err error) { result = &err })
	pumpUntil(t, a, func() bool { return result != nil })
	return *result
}

func (d *mockDesktop) openedURIs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.opened)
}

func TestOpenURLOpensThroughThePortal(t *testing.T) {
	d := startMockDesktop(t)
	a := launchApp(t)
	if err := openAndWait(t, a, a.OpenURL, "https://example.com"); err != nil {
		t.Fatalf("OpenURL = %v", err)
	}
	if err := openAndWait(t, a, a.OpenPath, "/tmp/report.pdf"); err != nil {
		t.Fatalf("OpenPath = %v", err)
	}
	if got, want := d.openedURIs(), []string{"https://example.com", "file:///tmp/report.pdf"}; !slices.Equal(got, want) {
		t.Errorf("the portal opened %q, want %q", got, want)
	}
}

func TestOpenURLRefusedByThePortalReachesTheCaller(t *testing.T) {
	d := startMockDesktop(t)
	a := launchApp(t)
	if err := openAndWait(t, a, a.OpenURL, refusedURI); !errors.Is(err, ErrPortalDenied) || !strings.Contains(err.Error(), refusedURI) {
		t.Fatalf("a refused open = %v, want ErrPortalDenied naming the URI", err)
	}
	if got := d.openedURIs(); !slices.Equal(got, []string{refusedURI}) {
		t.Errorf("the portal was asked %q, want one call", got)
	}
}

func TestOpenURLWithoutAPortalSaysSo(t *testing.T) {
	dbustest.Session(t)
	a := launchApp(t)
	if err := openAndWait(t, a, a.OpenURL, "https://example.com"); !errors.Is(err, ErrPortalUnavailable) {
		t.Fatalf("opening without a portal = %v, want ErrPortalUnavailable", err)
	}
}

func TestOpenURLWithoutAURIFails(t *testing.T) {
	d := startMockDesktop(t)
	a := launchApp(t)
	if err := openAndWait(t, a, a.OpenURL, ""); err == nil {
		t.Fatal("opening an empty URI succeeded")
	}
	if got := d.openedURIs(); len(got) != 0 {
		t.Errorf("the portal was asked %q for an empty URI", got)
	}
}
