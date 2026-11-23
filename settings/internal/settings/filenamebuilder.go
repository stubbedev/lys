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
	filenameDates    = []string{"", "%Y-%m-%d", "%Y%m%d", "%d-%m-%Y"}
	filenameTimes    = []string{"", "%H-%M-%S", "%H%M%S", "%H-%M", "%H.%M.%S"}
	filenameCounters = []string{"", config.ScreenshotCounter}
)

type filenameParts struct{ prefix, date, clock, counter string }

func (p filenameParts) format() string {
	stamp := slices.DeleteFunc([]string{p.date, p.clock, p.counter}, func(s string) bool { return s == "" })
	return p.prefix + strings.Join(stamp, dateTimeJoin) + screenshotExt
}

func (p filenameParts) fixed() bool { return p.date == "" && p.clock == "" && p.counter == "" }

func filenamePrefixProblem(prefix string) string {
	switch {
	case strings.ContainsAny(prefix, "/\x00"):
		return "settings-filename-slash"
	case strings.Contains(prefix, "%"):
		return "settings-filename-percent"
	case strings.Contains(prefix, config.ScreenshotCounter):
		return "settings-filename-counter-in-name"
	}
	return ""
}

func screenshotNameParts() formatKind {
	f := strftimeParts()
	f.render = renderScreenshotName
	f.insert = append(f.insert, choice{value: config.ScreenshotCounter, label: locale.T().Get("settings-filename-counter"), group: f.label(config.ScreenshotCounter)})
	return f
}

func renderScreenshotName(s string) (string, error) {
	f, err := config.ParseScreenshotFilename(s)
	if err != nil {
		return "", err
	}
	return f.At(formatSampleTime).Numbered(1), nil
}

func parseFilename(format string) (filenameParts, bool) {
	rest, ok := strings.CutSuffix(format, screenshotExt)
	if !ok {
		return filenameParts{}, false
	}
	var candidates []filenameParts
	for _, d := range filenameDates {
		for _, c := range filenameTimes {
			for _, n := range filenameCounters {
				candidates = append(candidates, filenameParts{date: d, clock: c, counter: n})
			}
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
		if p.prefix == "" && p.fixed() {
			return filenameParts{}, false
		}
		return p, true
	}
	return filenameParts{}, false
}

var screenshotNameRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, meta config.FieldMeta) control {
		return newFilenameEditor(k, s, k.store.slot(screenshotDirPath), newLabelBuilder(k, s, meta.Optional, screenshotNameParts()))
	}
}

type filenameEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	dir     slot
	simple  *widget.Box
	prefix  *widget.Entry
	date    *choicePick
	clock   *choicePick
	counter *choicePick
	problem *widget.Label
	path    *widget.Label
	notice  *widget.Label
	toggle  *modeToggle
	custom  *labelBuilder
	syncing bool
}

func choiceLabels(formats []string) []string {
	t := locale.T()
	out := make([]string, len(formats))
	for i, f := range formats {
		out[i] = cmp.Or(formatLabel(renderScreenshotName, f), t.Get("settings-filename-none"))
	}
	return out
}

func counterLabels() []string {
	t := locale.T()
	return []string{t.Get("settings-filename-none"), t.Get("settings-filename-counter-sample")}
}

func newFilenameEditor(k *kit, s, dir slot, custom *labelBuilder) *filenameEditor {
	t := locale.T()
	e := &filenameEditor{Box: gapBox(widget.Column), k: k, slot: s, dir: dir, custom: custom}
	e.prefix = k.entry()
	e.prefix.SetAccessibleName(t.Get("settings-filename-prefix"))
	e.prefix.SetPlaceholder(t.Get("settings-filename-prefix-placeholder"))
	e.prefix.OnChanged = func(string) { e.write() }
	e.date = newChoicePick(k, choiceLabels(filenameDates), 0)
	e.date.SetAccessibleName(t.Get("settings-filename-date"))
	e.date.OnSelect = func(int) { e.write() }
	e.clock = newChoicePick(k, choiceLabels(filenameTimes), 0)
	e.clock.SetAccessibleName(t.Get("settings-filename-time"))
	e.clock.OnSelect = func(int) { e.write() }
	e.counter = newChoicePick(k, counterLabels(), 0)
	e.counter.SetAccessibleName(t.Get("settings-filename-counter"))
	e.counter.OnSelect = func(int) { e.write() }
	e.simple = gapBox(widget.Column)
	for _, part := range []struct {
		key string
		w   widget.Widget
	}{{"settings-filename-prefix", e.prefix}, {"settings-filename-date", e.date}, {"settings-filename-time", e.clock}, {"settings-filename-counter", e.counter}} {
		line := gapBox(widget.Row)
		line.AppendAligned(k.label(t.Get(part.key), cardFormClasses.label, css.Caption), false, widget.AlignCenter)
		line.Append(part.w, true)
		e.simple.Append(line, false)
	}
	e.problem = k.errorText("")
	e.problem.SetVisible(false)
	e.path = k.note("", "format-preview")
	e.notice = k.note(t.Get("settings-filename-custom-notice"), "format-preview")
	e.toggle = k.textModeToggle(func(bool) { e.refresh() })
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
	return filenameParts{prefix: e.prefix.Text(), date: filenameDates[e.date.Selected()], clock: filenameTimes[e.clock.Selected()], counter: filenameCounters[e.counter.Selected()]}
}

func (e *filenameEditor) write() {
	if e.syncing {
		return
	}
	p := e.parts()
	key := filenamePrefixProblem(p.prefix)
	if key == "" && p.prefix == "" && p.fixed() {
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
		e.date.show(slices.Index(filenameDates, p.date))
		e.clock.show(slices.Index(filenameTimes, p.clock))
		e.counter.show(slices.Index(filenameCounters, p.counter))
		e.syncing = false
	}
	e.simple.SetVisible(simple)
	e.notice.SetVisible(!simple)
	e.toggle.SetVisible(simple)
	e.custom.SetVisible(!simple || e.toggle.isOn())
	name, err := renderScreenshotName(format)
	e.path.SetVisible(err == nil)
	full := filepath.Join(config.ScreenshotDirectory(e.dir.text(), systemHost.env), name)
	key := "settings-filename-path"
	switch {
	case strings.Contains(format, config.ScreenshotCounter):
		key = "settings-filename-path-counted"
	case simple && p.fixed():
		key = "settings-filename-path-numbered"
	}
	e.path.SetText(t.Get(key, i18n.Str("path", full)))
	e.custom.refresh()
}
