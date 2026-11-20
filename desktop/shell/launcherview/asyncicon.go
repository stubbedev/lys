package launcherview

import "github.com/stubbedev/lys/ui/widget"

// AsyncIcon is a fixed-size slot for an icon that may not be decoded yet.
// The slot is its size from the start, so an icon landing moves nothing,
// and the one Icon widget is renamed rather than rebuilt.
type AsyncIcon struct {
	Slot *widget.Box
	icon *widget.Icon
	size int
	// name is what the slot should show, shown what it does.
	name, shown string
}

// NewAsyncIcon is an empty slot size pixels square, of class.
func NewAsyncIcon(size int, class string) *AsyncIcon {
	a := &AsyncIcon{Slot: widget.NewBox(widget.Row, 0, 0), size: size}
	a.Slot.SetSizeRequest(size, size)
	a.Slot.AddClass(class)
	return a
}

// Size is the slot's side in pixels.
func (a *AsyncIcon) Size() int { return a.size }

// Name is the icon the slot should show, "" for none.
func (a *AsyncIcon) Name() string { return a.name }

// Shown is the icon the slot shows, "" while blank.
func (a *AsyncIcon) Shown() string { return a.shown }

// Want names the icon the slot should show, without showing it yet.
func (a *AsyncIcon) Want(name string) { a.name = name }

// Show puts the decoded icon in the slot. Only this touches a theme name
// on the UI loop, once the cache holds its raster.
func (a *AsyncIcon) Show(name string) {
	a.shown = name
	if a.icon == nil {
		a.icon = widget.NewThemeIconFallback(name, a.size)
		a.Slot.Append(a.icon, false)
		return
	}
	a.icon.SetThemeName(name)
	a.icon.SetVisible(true)
}

// Blank shows nothing in the slot, keeping its size.
func (a *AsyncIcon) Blank() {
	a.shown = ""
	if a.icon != nil {
		a.icon.SetVisible(false)
	}
}
