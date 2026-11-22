package settings

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	swallowTerminalsPath = "swallow.terminals"
	swallowExceptPath    = "swallow.except_title"
)

func wmSwallowRows() []rowSpec {
	return []rowSpec{
		wmField(swallowTerminalsPath, func(r *rowSpec) {
			r.fullWidth = true
			r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newSwallowTerminals(k, s) }
		}),
		wmField(swallowExceptPath, enabledWhile(swallowTerminalsPath, swallowing)),
	}
}

func swallowing(v any) bool {
	pattern, _ := v.(string)
	return pattern != ""
}

// terminalsPattern is the app_id pattern matching exactly the apps ids.
func terminalsPattern(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = regexp.QuoteMeta(id)
	}
	return "^(" + strings.Join(quoted, "|") + ")$"
}

// terminalIDs are the app ids a pattern matches exactly, when it is one
// terminalsPattern writes or the single ^id$; false for any pattern a list
// of apps cannot say.
func terminalIDs(pattern string) ([]string, bool) {
	inner, ok := strings.CutPrefix(pattern, "^")
	if !ok {
		return nil, false
	}
	if inner, ok = strings.CutSuffix(inner, "$"); !ok {
		return nil, false
	}
	if group, grouped := strings.CutPrefix(inner, "("); grouped {
		if inner, ok = strings.CutSuffix(group, ")"); !ok {
			return nil, false
		}
	}
	alternatives := strings.Split(inner, "|")
	ids := make([]string, 0, len(alternatives))
	for _, alt := range alternatives {
		id, ok := unquoteMeta(alt)
		if !ok || id == "" || slices.Contains(ids, id) {
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func unquoteMeta(quoted string) (string, bool) {
	var b strings.Builder
	escaped := false
	for _, r := range quoted {
		if r == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false
		b.WriteRune(r)
	}
	id := b.String()
	return id, !escaped && regexp.QuoteMeta(id) == quoted
}

var terminalsSpec = appStripSpec{
	class: "swallow-terminals", none: "wm-swallow-terminals-none", add: "wm-swallow-terminals-add",
	offer: func(ix installedApps) []desktopentry.App {
		return slices.DeleteFunc(slices.Clone(ix.shown), func(a desktopentry.App) bool { return !isTerminal(a) })
	},
}

// swallowTerminals is the terminals whose windows a window they start takes
// the place of: installed terminals picked into a list, written as the
// pattern matching exactly their app ids. A pattern written by hand that no
// list says is shown as it is and kept until it is replaced by a list.
type swallowTerminals struct {
	*widget.Box
	w       *wmBackend
	slot    slot
	strip   *appStrip
	written *widget.Box
	pattern *widget.Label
}

func newSwallowTerminals(k *kit, s slot) *swallowTerminals {
	t := locale.T()
	c := &swallowTerminals{Box: gapBox(widget.Column), w: k.wm, slot: s}
	ids, _ := terminalIDs(s.text())
	c.strip = newAppStrip(k, terminalsSpec, ids, c.write)
	c.written = gapBox(widget.Row)
	c.written.AddClass("swallow-terminals-pattern")
	c.pattern = wrapped(k.label("", "swallow-terminals-written", css.Monospace))
	c.pattern.SetTooltip(t.Get("wm-swallow-terminals-written"))
	c.written.AppendAligned(c.pattern, true, widget.AlignCenter)
	c.written.AppendAligned(k.secondaryButton(t.Get("wm-swallow-terminals-replace"), c.replace), false, widget.AlignCenter)
	c.Append(c.strip, false)
	c.Append(c.written, false)
	c.refresh()
	return c
}

func (c *swallowTerminals) stacked() {}

func (c *swallowTerminals) write(ids []string) {
	if len(ids) == 0 {
		c.w.clear(c.slot, swallowTerminalsPath)
		return
	}
	_ = c.slot.set(terminalsPattern(ids))
}

func (c *swallowTerminals) replace() {
	c.w.clear(c.slot, swallowTerminalsPath)
	c.refresh()
}

func (c *swallowTerminals) refresh() {
	raw := c.slot.get()
	pattern, isText := raw.(string)
	ids, listed := terminalIDs(pattern)
	if raw == nil || pattern == "" && isText {
		ids, listed = nil, true
	}
	c.strip.SetVisible(listed)
	c.written.SetVisible(!listed)
	if listed {
		c.strip.show(ids)
		return
	}
	c.pattern.SetText(fmt.Sprint(raw))
}
