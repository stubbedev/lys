package widget

var (
	errorBell   perLoop[func(Widget)]
	surfaceFade perLoop[func(Widget, float64) bool]
)

// SetSurfaceFade installs fn, which sets the opacity the compositor
// applies to the surface whose root is root and reports whether it
// did: false when root is not a surface's root or the compositor
// cannot fade surfaces. A Revealer at its surface's root fades through
// it instead of repainting its child every frame. It serves the
// Revealers animated on the calling goroutine, the loop of the
// application that installs it. nil removes the calling goroutine's.
// The returned function removes this one and leaves a later install in
// place.
func SetSurfaceFade(fn func(root Widget, alpha float64) bool) (remove func()) {
	return installHook(&surfaceFade, fn, fn == nil)
}

func fadeSurface(root Widget, alpha float64) bool {
	fn := surfaceFade.current()
	return fn != nil && (*fn)(root, alpha)
}

// SetErrorBell installs the bell ErrorBell rings on the calling
// goroutine, the loop of the application whose windows it rings; nil
// silences the calling goroutine's. The returned function removes this
// one and leaves a later install in place.
func SetErrorBell(fn func(w Widget)) (remove func()) {
	return installHook(&errorBell, fn, fn == nil)
}

// ErrorBell signals an input w rejected (gtk_widget_error_bell):
// typing into a read-only field, a value that does not parse. The
// application rings the system bell for w's window; the compositor may
// flash it instead.
func ErrorBell(w Widget) {
	if fn := errorBell.current(); fn != nil {
		(*fn)(w)
	}
}

// RootOf is the top of w's tree: the widget with no parent above it.
func RootOf(w Widget) Widget {
	for {
		p := parentOf(w)
		if p == nil {
			return w
		}
		w = p
	}
}
