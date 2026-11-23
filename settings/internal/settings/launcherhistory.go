package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const clearHistoryKey = "settings-launcher-clear-history"

// clearLauncherHistoryArgs is the lys-shell command that forgets every
// launch the launcher ranks by.
var clearLauncherHistoryArgs = []string{"launcher", "-clear-history"}

// clearHistoryRow asks before lys-shell forgets the launcher's history.
func clearHistoryRow() rowSpec {
	return field("launcher.history.enable", withKey(clearHistoryKey), func(r *rowSpec) {
		r.action = true
		r.editor = func(k *kit, _ slot, _ config.FieldMeta) control { return newClearHistory(k) }
	})
}

type clearHistory struct {
	*widget.Box
	k       *kit
	button  *widget.Button
	outcome *inlineState
}

func newClearHistory(k *kit) *clearHistory {
	c := &clearHistory{Box: gapBox(widget.Row), k: k, outcome: k.inlineState()}
	c.button = k.destructiveButton(locale.T().Get(clearHistoryKey+"-button"), c.ask)
	c.AppendAligned(c.button, false, widget.AlignCenter)
	c.Append(c.outcome, true)
	return c
}

func (c *clearHistory) refresh() {}

func (c *clearHistory) ask() {
	if err := c.k.confirm("clear the launcher history", clearHistoryKey, c.clear); err != nil {
		c.show(err)
	}
}

func (c *clearHistory) clear() {
	c.button.SetEnabled(false)
	async(c.k, func() error { return c.k.shellCommand(clearLauncherHistoryArgs...) }, func(err error) {
		c.button.SetEnabled(true)
		c.show(err)
	})
}

func (c *clearHistory) show(err error) {
	t := locale.T()
	if err != nil {
		c.outcome.Error(t.Get(clearHistoryKey+"-failed"), err, nil)
		return
	}
	c.outcome.Empty("", t.Get(clearHistoryKey+"-done"))
}
