// Package layers names the layer surfaces lys-shell maps, as the
// compositor's layer rules match them by namespace.
package layers

import "regexp"

// Surface is a layer surface lys-shell maps under one namespace.
type Surface string

// OutputSurface is a layer surface lys-shell maps on each output, its
// namespace this prefix and the output's name.
type OutputSurface string

// Kind is a surface lys-shell maps, as a layer rule names it.
type Kind interface {
	// Name is the namespace, or for one per output its prefix.
	Name() string
	// Pattern is the regular expression matching every namespace of it.
	Pattern() string
}

// The surfaces lys-shell maps.
const (
	Bar                 OutputSurface = "lys-shell-bar"
	Dock                OutputSurface = "lys-shell-dock"
	Wallpaper           Surface       = "lys-shell-wallpaper"
	Launcher            Surface       = "lys-shell-launcher"
	NotificationPopup   Surface       = "lys-shell-notification-popup"
	OSD                 Surface       = "lys-shell-osd"
	PowerMenu           Surface       = "lys-shell-power-menu"
	PortalDialog        Surface       = "lys-shell-portal-dialog"
	FileChooser         Surface       = "lys-shell-file-chooser"
	Print               Surface       = "lys-shell-print"
	SharePicker         Surface       = "lys-shell-share-picker"
	ScreenshotReading   Surface       = "lys-shell-screenshot-reading"
	ScreenshotThumbnail Surface       = "lys-shell-screenshot-thumbnail"
	ColorPicker         Surface       = "lys-shell-color-picker"
	RegionOverlay       Surface       = "lys-shell-region-overlay"
	WindowPick          Surface       = "lys-shell-window-pick"
	QuickLook           Surface       = "lys-shell-quick-look"
)

// All are the surfaces lys-shell maps, the panels first.
var All = []Kind{
	Bar, Dock, Wallpaper, Launcher, NotificationPopup, OSD, PowerMenu, PortalDialog, FileChooser, Print,
	SharePicker, ScreenshotReading, ScreenshotThumbnail, ColorPicker, RegionOverlay, WindowPick,
	QuickLook,
}

// Name is the surface's namespace.
func (s Surface) Name() string { return string(s) }

// Pattern matches the surface's namespace exactly.
func (s Surface) Pattern() string { return "^" + regexp.QuoteMeta(string(s)) + "$" }

// Name is the namespaces' prefix.
func (s OutputSurface) Name() string { return string(s) }

// On is the namespace of the surface on the output named output.
func (s OutputSurface) On(output string) string { return string(s) + "-" + output }

// Pattern matches the surface's namespace on every output.
func (s OutputSurface) Pattern() string { return "^" + regexp.QuoteMeta(string(s)+"-") }
