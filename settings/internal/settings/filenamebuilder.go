package settings

import (
	"cmp"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	screenshotDirPath = "modules.screenshot.output-directory"
	screenshotExt     = ".png"
	dateTimeJoin      = "_"
)

var (
	filenameDates = []string{"", "%Y-%m-%d", "%Y%m%d", "%d-%m-%Y"}
	filenameTimes = []string{"", "%H-%M-%S", "%H%M%S", "%H-%M", "%H.%M.%S"}
)

type filenameParts struct{ prefix, date, clock string }

func (p filenameParts) format() string {
	join := ""
	if p.date != "" && p.clock != "" {
		join = dateTimeJoin
	}
	return p.prefix + p.date + join + p.clock + screenshotExt
}

func (p filenameParts) numbered() bool { return p.date == "" && p.clock == "" }

func filenamePrefixProblem(prefix string) string {
	switch {
	case strings.ContainsAny(prefix, "/\x00"):
		return "settings-filename-slash"
	case strings.Contains(prefix, "%"):
		return "settings-filename-percent"
	}
	return ""
}

func parseFilename(format string) (filenameParts, bool) {
	rest, ok := strings.CutSuffix(format, screenshotExt)
	if !ok {
		return filenameParts{}, false
	}
	var candidates []filenameParts
	for _, d := range filenameDates {
		for _, c := range filenameTimes {
			candidates = append(candidates, filenameParts{date: d, clock: c})
		}
	}
	slices.SortStableFunc(candidates, func(a, b filenameParts) int { return len(b.format()) - len(a.format()) })
	for _, p := range candidates {
		tail := strings.TrimSuffix(p.format(), screenshotExt)
		prefix, ok := strings.CutSuffix(rest, tail)
		if !ok || filenamePrefixProblem(prefix) != "" {
			continue
		}
		p.prefix = prefix
		if p.prefix == "" && p.numbered() {
			return filenameParts{}, false
		}
		return p, true
	}
	return filenameParts{}, false
}

var screenshotNameRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
		return newFilenameEditor(k, s, k.store.slot(screenshotDirPath), newLabelBuilder(k, s, meta.Optional, strftimeParts()))
	}
}

type filenameEditor struct {
	*widget.Box
	k        *kit
	slot     slot
	dir      slot
	simple   *widget.Box
	prefix   *widget.Entry
	date     *widget.Dropdown
	clock    *widget.Dropdown
	problem  *widget.Label
	path     *widget.Label
	notice   *widget.Label
	toggle   *widget.Button
	toggled  *widget.Label
	custom   *labelBuilder
	advanced bool
	syncing  bool
}

func choiceLabels(formats []string) []string {
	t := locale.T()
	render := strftimeFormat().render
	out := make([]string, len(formats))
	for i, f := range formats {
		out[i] = cmp.Or(render(f), t.Get("settings-filename-none"))
	}
	return out
}

func newFilenameEditor(k *kit, s, dir slot, custom *labelBuilder) *filenameEditor {
	t := locale.T()
	e := &filenameEditor{Box: gapBox(widget.Column), k: k, slot: s, dir: dir, custom: custom}
	e.prefix = k.entry()
	e.prefix.SetAccessibleName(t.Get("settings-filename-prefix"))
	e.prefix.SetPlaceholder(t.Get("settings-filename-prefix-placeholder"))
	e.prefix.OnChanged = func(string) { e.write() }
	e.date = widget.NewDropdown(k.face, k.textPx(), choiceLabels(filenameDates), 0)
	e.date.SetAccessibleName(t.Get("settings-filename-date"))
	e.date.OnSelect = func(int) { e.write() }
	e.clock = widget.NewDropdown(k.face, k.textPx(), choiceLabels(filenameTimes), 0)
	e.clock.SetAccessibleName(t.Get("settings-filename-time"))
	e.clock.OnSelect = func(int) { e.write() }
	e.simple = gapBox(widget.Column)
	for _, part := range []struct {
		key string
		w   widget.Widget
	}{{"settings-filename-prefix", e.prefix}, {"settings-filename-date", e.date}, {"settings-filename-time", e.clock}} {
		line := gapBox(widget.Row)
		line.AppendAligned(k.label(t.Get(part.key), "card-form-label", css.Caption), false, widget.AlignCenter)
		line.Append(part.w, true)
		e.simple.Append(line, false)
	}
	e.problem = wrapped(k.label("", "format-preview", css.Caption, css.Error))
	e.problem.SetVisible(false)
	e.path = wrapped(k.label("", "format-preview", css.Caption, css.DimLabel))
	e.notice = wrapped(k.label(t.Get("settings-filename-custom-notice"), "format-preview", css.Caption, css.DimLabel))
	e.toggled = k.label(t.Get("settings-filename-show-custom"))
	e.toggle = k.button(e.toggled, func() {
		e.advanced = !e.advanced
		e.refresh()
	}, css.Small, css.Flat)
	e.Append(e.simple, false)
	e.Append(e.problem, false)
	e.Append(e.path, false)
	e.Append(e.notice, false)
	e.AppendAligned(e.toggle, false, widget.AlignStart)
	e.Append(e.custom, false)
	e.refresh()
	return e
}

func (e *filenameEditor) stacked() {}

func (e *filenameEditor) dirtyBadge() *widget.Badge { return e.custom.dirtyBadge() }

func (e *filenameEditor) parts() filenameParts {
	return filenameParts{prefix: e.prefix.Text(), date: filenameDates[e.date.Selected()], clock: filenameTimes[e.clock.Selected()]}
}

func (e *filenameEditor) write() {
	if e.syncing {
		return
	}
	p := e.parts()
	key := filenamePrefixProblem(p.prefix)
	if key == "" && p.prefix == "" && p.numbered() {
		key = "settings-filename-empty"
	}
	if key != "" {
		e.problem.SetText(locale.T().Get(key))
		e.problem.SetVisible(true)
		return
	}
	e.problem.SetVisible(false)
	if e.slot.set(p.format()) != nil {
		return
	}
	e.refresh()
}

func (e *filenameEditor) refresh() {
	t := locale.T()
	format := e.slot.text()
	p, simple := parseFilename(format)
	if simple {
		e.syncing = true
		if e.prefix.Text() != p.prefix {
			e.prefix.SetText(p.prefix)
		}
		e.date.SetSelected(slices.Index(filenameDates, p.date))
		e.clock.SetSelected(slices.Index(filenameTimes, p.clock))
		e.syncing = false
	}
	e.simple.SetVisible(simple)
	e.notice.SetVisible(!simple)
	e.toggle.SetVisible(simple)
	e.custom.SetVisible(!simple || e.advanced)
	toggleKey := "settings-filename-show-custom"
	if e.advanced {
		toggleKey = "settings-filename-hide-custom"
	}
	e.toggled.SetText(t.Get(toggleKey))
	full := filepath.Join(config.ScreenshotDirectory(e.dir.text(), systemHost.env), strftimeFormat().render(format))
	key := "settings-filename-path"
	if simple && p.numbered() {
		key = "settings-filename-path-numbered"
	}
	e.path.SetText(t.Get(key, i18n.Str("path", full)))
	e.custom.refresh()
}
