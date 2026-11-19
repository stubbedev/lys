package barview

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

// IconPx is the bar's icon size in logical pixels; the label height
// scales around it.
const IconPx = 16

// Icon builds the theme icon a module shows beside its label; nil when
// the module shows none or the name is empty. The glyph is the bar
// button's image: its color and size come from the stylesheet
// (--bar-btn-icon-color, --bar-btn-icon-size). The concrete return
// type keeps nil checks honest: a nil *widget.Icon never hides inside a
// non-nil interface.
func Icon(icon config.IconConfig) *widget.Icon {
	if !icon.Show || icon.Name == "" {
		return nil
	}
	return widget.NewThemeIcon(icon.Name, IconPx)
}
