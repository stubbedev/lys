package bar

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"
)

// barButton is a module's barview.Button with the module's five click
// and scroll bindings: the button is the router's hit leaf, so the
// bindings route through it.
type barButton struct {
	*barview.Button

	binding config.ClickConfig
	onRun   func(action config.ClickAction, scroll bool)
}

// newBarButton builds a module's bar button of its icon (nil for none)
// and its label text (empty for none yet).
func newBarButton(ctx ModuleContext, icon *widget.Icon, label string) *barButton {
	return withBindings(barview.NewButton(ctx.Config, ctx.Font, icon, label))
}

// newBarButtonAround wraps arbitrary module content (a canvas, a row of
// its own) in the same button, in the icon's place.
func newBarButtonAround(ctx ModuleContext, content widget.Widget) *barButton {
	return withBindings(barview.NewButtonAround(ctx.Config, ctx.Font, content))
}

func withBindings(button *barview.Button) *barButton {
	b := &barButton{Button: button}
	b.SetOnPointer(b.pointer)
	b.SetOnScroll(b.scroll)
	return b
}

// configure applies a module's title, button config and bindings;
// appendModule calls it once it knows which module the button belongs
// to.
func (b *barButton) configure(title string, button config.ButtonConfig, binding config.ClickConfig, onRun func(action config.ClickAction, scroll bool)) {
	b.binding, b.onRun = binding, onRun
	b.Configure(title, button)
}

// run fires one binding; scroll marks the scroll-up/down ones (the custom
// module debounces its on-action for those).
func (b *barButton) run(action config.ClickAction, scroll bool) {
	if action.Kind == config.ClickNone || b.onRun == nil {
		return
	}
	b.onRun(action, scroll)
}

// bound reports whether any of the five bindings is set.
func (b *barButton) bound() bool {
	c := b.binding
	return c.LeftClick.Kind != config.ClickNone || c.MiddleClick.Kind != config.ClickNone ||
		c.RightClick.Kind != config.ClickNone || c.ScrollUp.Kind != config.ClickNone ||
		c.ScrollDown.Kind != config.ClickNone
}

// HitTest keeps the bar button the hit leaf, so the router drives its
// hover, press, and click, and the non-primary hooks below.
func (b *barButton) HitTest(p widget.Point) widget.Widget {
	if b.BarButton.HitTest(p) != nil {
		return b
	}
	return nil
}

// SetPressed fires the primary binding on the press: the dropdown appears under the
// button-down, and a press that slides off before the release still
// opened it. The release's click opens nothing: the button has no
// popover of its own, the binding opens the dropdown.
func (b *barButton) SetPressed(on bool) {
	was := b.Pressed
	b.BarButton.SetPressed(on)
	if on && !was {
		b.run(b.binding.LeftClick, false)
	}
}

// KeyAction runs the primary binding on Enter, as a press does: the bar
// walked by keyboard opens a module's dropdown from its focused button.
func (b *barButton) KeyAction(a widget.KeyAction, _ widget.Mods) {
	if a == widget.KeyEnter {
		b.run(b.binding.LeftClick, false)
	}
}

// InsertRune runs the primary binding on Space, the other half of the
// activation pair.
func (b *barButton) InsertRune(r rune) {
	if r == ' ' {
		b.run(b.binding.LeftClick, false)
	}
}

// pointer runs the middle and right bindings and consumes those
// presses; the primary goes on to the button's press.
func (b *barButton) pointer(e widget.PointerEvent) bool {
	switch e.Button {
	case widget.BTNMiddle:
		b.run(b.binding.MiddleClick, false)
	case widget.BTNRight:
		b.run(b.binding.RightClick, false)
	default:
		return false
	}
	return true
}

// scroll maps vertical steps to the up/down bindings, positive DY
// scrolling down. The step is always consumed: an unbound action no-ops in run, and content
// behind the bar never scrolls through a module.
func (b *barButton) scroll(e widget.ScrollEvent) bool {
	switch {
	case e.DY > 0:
		b.run(b.binding.ScrollDown, true)
	case e.DY < 0:
		b.run(b.binding.ScrollUp, true)
	default:
		return false
	}
	return true
}
