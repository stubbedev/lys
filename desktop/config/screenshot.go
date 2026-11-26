package config

import (
	"path/filepath"

	"github.com/stubbedev/lys/lib/xdg"
)

// ScreenshotConfig configures the screenshot module.
//
// Screenshot capture button.
//
// Click the bar button to capture a region, output, or window. Controllable
// from the CLI / RPC socket: `lys-shell screenshot region|output|window`.
//
// After a capture a thumbnail floats in a screen corner: click it to mark
// the image up (crop, arrow, rectangle, ellipse, freehand, text,
// highlighter, numbered markers and pixelation, with undo and redo; Enter
// saves over the file and copies it, Ctrl+S saves a copy elsewhere), drag
// it into an app, or delete the capture with its close button.
//
// `lys-shell screenshot text` copies the text in a dragged region (Latin
// script, drawn in the UI's fonts or the installed default ones) and
// `lys-shell screenshot qr` decodes the QR code in one; neither saves a
// file.
type ScreenshotConfig struct {
	// Bar button icon.
	Icon string `cfg:"icon"`
	// Output directory for screenshots. Empty uses the XDG Pictures directory.
	OutputDirectory string `cfg:"output-directory,type=Directory"`
	// Saved file name, formatted with `chrono`/`strftime` specifiers.
	// `{counter}` numbers the files 1, 2, 3 and on, one past the highest
	// number already saved under the same name; a name without it gets
	// -2, -3 and on when it is taken.
	FilenameFormat ScreenshotFilename `cfg:"filename-format"`
	// Copy the captured image to the clipboard.
	CopyToClipboard bool `cfg:"copy-to-clipboard"`
	// Show a desktop notification after capturing. With the thumbnail
	// disabled it carries Edit, Open folder and Delete buttons.
	Notify bool `cfg:"notify"`
	// How long, in milliseconds, the floating thumbnail stays after a
	// capture; hovering it pauses the countdown. Click it to annotate, drag
	// it into an app, or delete the capture from it. Set to 0 to disable.
	ThumbnailMs uint32 `cfg:"thumbnail-ms"`
	// Screen corner the floating thumbnail sits in.
	ThumbnailPosition ScreenshotCorner `cfg:"thumbnail-position"`
	// Display border around button.
	BorderShow bool `cfg:"border-show"`
	// Border color token.
	BorderColor ColorValue `cfg:"border-color"`
	// Display module icon.
	IconShow bool `cfg:"icon-show"`
	// Show the module's state in a tooltip on hover.
	Tooltip bool `cfg:"tooltip"`
	// Icon foreground color. Auto selects based on variant for contrast.
	IconColor ColorValue `cfg:"icon-color"`
	// Icon container background color token.
	IconBgColor ColorValue `cfg:"icon-bg-color"`
	// Static label text shown beside the icon.
	Label string `cfg:"label"`
	// Display label.
	LabelShow bool `cfg:"label-show"`
	// Label text color token.
	LabelColor ColorValue `cfg:"label-color"`
	// Max label characters before truncation with ellipsis. Set to 0 to disable.
	LabelMaxLength uint32 `cfg:"label-max-length"`
	// Button background color token.
	ButtonBgColor ColorValue `cfg:"button-bg-color"`
	// Action on left click. Default captures a region.
	LeftClick ClickAction `cfg:"left-click"`
	// Action on right click. Default captures the focused output.
	RightClick ClickAction `cfg:"right-click"`
	// Action on middle click. Default captures the active window.
	MiddleClick ClickAction `cfg:"middle-click"`
	// Action on scroll up.
	ScrollUp ClickAction `cfg:"scroll-up"`
	// Action on scroll down.
	ScrollDown ClickAction `cfg:"scroll-down"`
}

// ScreenshotDirectory is the directory screenshots are saved in: the
// configured one, else $XDG_PICTURES_DIR, else ~/Pictures, else the
// working directory.
func ScreenshotDirectory(configured string, env xdg.Env) string {
	if configured != "" {
		return configured
	}
	if dir := env("XDG_PICTURES_DIR"); dir != "" {
		return dir
	}
	if home := env("HOME"); home != "" {
		return filepath.Join(home, "Pictures")
	}
	return "."
}

// DefaultsScreenshot returns the schema defaults.
func DefaultsScreenshot() ScreenshotConfig {
	return ScreenshotConfig{
		Icon:              "ld-camera-symbolic",
		OutputDirectory:   "",
		FilenameFormat:    mustScreenshotFilename("Screenshot_%Y-%m-%d_%H-%M-%S.png"),
		CopyToClipboard:   true,
		Notify:            true,
		ThumbnailMs:       5000,
		ThumbnailPosition: ScreenshotBottomRight,
		BorderShow:        false,
		BorderColor:       mustColor("accent"),
		IconShow:          true,
		Tooltip:           true,
		IconColor:         mustColor("auto"),
		IconBgColor:       mustColor("accent"),
		Label:             "",
		LabelShow:         false,
		LabelColor:        mustColor("fg-default"),
		LabelMaxLength:    0,
		ButtonBgColor:     mustColor("bg-surface-elevated"),
		LeftClick:         ParseClickAction("lys-shell screenshot region"),
		RightClick:        ParseClickAction("lys-shell screenshot output"),
		MiddleClick:       ParseClickAction("lys-shell screenshot window"),
		ScrollUp:          ClickAction{},
		ScrollDown:        ClickAction{},
	}
}

// Clicks returns the five input bindings.
func (c ScreenshotConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}

// ScreenshotCorner is the screen corner the capture thumbnail sits in.
//
// Screen corner for the screenshot thumbnail.
type ScreenshotCorner string

// Screenshot thumbnail corners.
const (
	// Top-left corner.
	ScreenshotTopLeft ScreenshotCorner = "top-left"
	// Top-right corner.
	ScreenshotTopRight ScreenshotCorner = "top-right"
	// Bottom-right corner.
	ScreenshotBottomRight ScreenshotCorner = "bottom-right"
	// Bottom-left corner.
	ScreenshotBottomLeft ScreenshotCorner = "bottom-left"
)

var _ = registerEnum(ScreenshotTopLeft, ScreenshotTopRight, ScreenshotBottomRight, ScreenshotBottomLeft)
