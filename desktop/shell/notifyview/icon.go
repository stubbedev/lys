package notifyview

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/shell/widgets"
)

// FallbackIcon is the bell every unresolved notification shows.
const FallbackIcon = "ld-bell-symbolic"

// lookIcon is the glyph of the button previewing an attachment.
const lookIcon = "ld-eye-symbolic"

// Icon is a resolved notification icon: a theme name or an image file
// (ResolvedIcon). Exactly one is set.
type Icon struct {
	Name string
	File string
}

// NewIcon builds the widget for a resolved icon at size pixels: a file
// image, decoded off the loop with the generic glyph until post delivers it, or
// a theme icon tinted when its name is symbolic. ready runs once a file shows.
func NewIcon(icon Icon, size int, tint render.Color, post func(func()), ready func()) *widget.Icon {
	if icon.File != "" {
		return widgets.NewAsyncFileIcon(icon.File, FallbackIcon, size, post, ready)
	}
	w := widget.NewThemeIconFallback(icon.Name, size)
	if strings.HasSuffix(icon.Name, "-symbolic") {
		w.SetTint(tint)
	}
	return w
}

// IsFileIcon reports whether an icon draws as a picture rather than a
// recolored glyph (an image, or a theme name that is not -symbolic):
// the card's icon box takes the file-icon class for it.
func IsFileIcon(icon Icon) bool {
	return icon.File != "" || !strings.HasSuffix(icon.Name, "-symbolic")
}

// Attachment is the picture a notification attaches that its icon
// shows: the icon's file when it is imagePath's, the image-path hint
// or the cached PNG of inline image data; "" when the icon shows
// anything else.
func Attachment(icon Icon, imagePath string) string {
	if icon.File == "" || strings.TrimPrefix(imagePath, "file://") != icon.File {
		return ""
	}
	return icon.File
}

// LookButton is the button previewing path, the picture a
// notification attaches, in Quick Look through look, its glyph size
// pixels and the file's name its tooltip; nil without a path or a look.
func LookButton(path string, look func(path string), size int) *widget.Button {
	if path == "" || look == nil {
		return nil
	}
	b := widget.NewButton(widget.NewThemeIcon(lookIcon, size), 0, 0)
	b.AddClass("notification-look", css.Ghost, css.Icon, css.Small)
	b.SetTooltip(filepath.Base(path))
	b.OnClick = func() { look(path) }
	return b
}

// Looker is a Look over look, Quick Look's, which shows a file until it
// closes: a failure to show is logged as what's. Nil without look.
func Looker(what string, look func(path string, closed func()) error) func(path string) {
	if look == nil {
		return nil
	}
	return func(path string) {
		if err := look(path, func() {}); err != nil {
			log.Printf("%s: quick look %s: %v", what, path, err)
		}
	}
}
