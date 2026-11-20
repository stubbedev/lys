package notifyview

import (
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/shell/widgets"
)

// FallbackIcon is the bell every unresolved notification shows.
const FallbackIcon = "ld-bell-symbolic"

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
