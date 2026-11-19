package bar

import (
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/ui/widget"
)

// separatorModule is the separator rule (barview.NewSeparator).
type separatorModule struct {
	rule *widget.BarSeparator
}

func newSeparator(ctx ModuleContext) (Module, error) {
	return &separatorModule{rule: barview.NewSeparator(ctx.Config)}, nil
}

func (m *separatorModule) Root() widget.Widget { return m.rule }

// ownsChrome keeps the separator out of the module-button wrapper: a
// rule is not a button.
func (m *separatorModule) ownsChrome() {}
