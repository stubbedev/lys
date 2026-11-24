package app

import (
	"errors"
	"fmt"

	"github.com/stubbedev/lys/lib/debug"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/third_party/wayland/wl"

	"github.com/stubbedev/lys/ui/transfer"
)

// External launching (#86): app.OpenURL and app.OpenPath hand a URL or
// a file to the user's handlers - the browser for https links, the
// file's app for files - the GTK UriLauncher/FileLauncher pair.
//
// Transport: the xdg-desktop-portal OpenURI interface, the one opener;
// without a portal nothing opens and the caller hears
// ErrPortalUnavailable. The portal call carries an xdg-activation token
// requested from the session at call time, so the launched app may take
// focus per the compositor's focus-stealing rules; a session without
// the activation protocol launches without one.

const portalOpenURI = "org.freedesktop.portal.OpenURI.OpenURI"

// launcher is the Application's launch state: the transport seam tests
// substitute, set once at construction to the portal's OpenURI and only
// read after.
type launcher struct {
	openURI func(parentWindow, uri, token string) error
}

// OpenURL opens uri in the user's handler for its scheme. It returns at
// once, and done (nil logs a failure instead) runs on the loop with nil
// once the portal opened it, ErrPortalDenied when the user or the
// desktop refused, ErrPortalUnavailable without an OpenURI portal, or
// the failure. The compositor decides focus, guided by an activation
// token when the session's xdg-activation support issued one.
func (a *Application) OpenURL(uri string, done func(error)) {
	a.launch(uri, done)
}

// OpenPath opens a file or directory in the user's handler for it, the
// FileLauncher shape; done is OpenURL's.
func (a *Application) OpenPath(path string, done func(error)) {
	a.launch(transfer.FileURI(path), done)
}

// launch routes one URI: with activation support the open waits for
// its own token (requested on the loop, anchored to the focused
// surface and the latest input event, so it vouches for a real
// interaction), else it opens now.
func (a *Application) launch(uri string, done func(error)) {
	if uri == "" {
		a.reportDone("open", done, errors.New("app: open: no URI to open"))
		return
	}
	if a.sess == nil || !a.sess.ActivationAvailable() {
		a.launchNow(uri, "", done)
		return
	}
	a.RequestActivationToken(func(token string, err error) {
		if err != nil {
			debug.Log("shell", "launch %s without an activation token: %v", uri, err)
		}
		a.launchNow(uri, token, done)
	})
}

// focusSurface is the focused window's surface, for anchoring the
// activation token.
func (a *Application) focusSurface() *wl.Surface {
	if hw := a.focused(); hw != nil && hw.host != nil {
		return hw.host.HostSurface()
	}
	return nil
}

// launchNow runs the transport off the caller's goroutine and reports
// its verdict to done. The focused-surface read happens on the caller's
// side - loop state never leaves the loop goroutine.
func (a *Application) launchNow(uri, token string, done func(error)) {
	open := a.launchState.openURI
	parent := ""
	if a.sess != nil {
		if hw := a.focused(); hw != nil {
			parent = portalParent(hw.win)
		}
	}
	go func() {
		a.reportDone("open "+uri, done, open(parent, uri, token))
	}()
}

// portalOpenURI opens uri through the OpenURI portal and waits for its
// verdict.
func (a *Application) portalOpenURI(parentWindow, uri, token string) error {
	options := map[string]dbus.Variant{}
	if token != "" {
		options["activation_token"] = dbus.MakeVariant(token)
	}
	resp, _, err := a.desktop().request(portalOpenURI, options, parentWindow, uri)
	if err != nil {
		return fmt.Errorf("app: open %s: %w", uri, err)
	}
	if resp.code != 0 {
		return fmt.Errorf("app: open %s: %w", uri, ErrPortalDenied)
	}
	return nil
}
