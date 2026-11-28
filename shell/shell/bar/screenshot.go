package bar

import (
	"errors"

	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"
)

// screenshotModule is the screenshot capture button: an icon and an optional
// static label whose bindings run `lys-shell screenshot region|output|window`
// (in-process through the builtin) by default.
type screenshotModule struct {
	btn *barButton
}

func newScreenshot(ctx ModuleContext) (Module, error) {
	cfg := ctx.Config.Screenshot
	label := ""
	if cfg.LabelShow {
		label = cfg.Label
	}
	icon := barview.Icon(cfg.IconView())
	if icon == nil && label == "" {
		return nil, errors.New("screenshot: neither the icon nor a label is shown")
	}
	return &screenshotModule{btn: newBarButton(ctx, icon, label)}, nil
}

func (m *screenshotModule) Root() widget.Widget { return m.btn }
