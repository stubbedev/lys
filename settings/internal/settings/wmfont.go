package settings

import (
	"fmt"
	"os"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type readFamily struct {
	path, family string
	err          error
}

type resolvedFont struct {
	family, path string
	err          error
}

type wmFontEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	picker  *fontEditor
	browse  *widget.Button
	file    *widget.Label
	problem *widget.Label
	read    string
	family  string
}

func newWMFontEditor(k *kit, s slot) *wmFontEditor {
	c := &wmFontEditor{Box: gapBox(widget.Column), k: k, slot: s}
	c.picker = newFontEditor(k, slot{get: func() any { return c.family }, set: c.pick, unset: s.unset}, false)
	head := gapBox(widget.Row)
	head.AppendAligned(c.picker, true, widget.AlignStart)
	c.browse = k.browseButton(func(path string) { _ = c.slot.set(path) })
	head.AppendAligned(c.browse, false, widget.AlignStart)
	c.file = wrapped(k.label("", "font-file", css.Caption, css.DimLabel))
	c.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	c.Append(head, false)
	c.Append(c.file, false)
	c.Append(c.problem, false)
	c.refresh()
	return c
}

func (c *wmFontEditor) stacked() {}

func (c *wmFontEditor) refresh() {
	path := c.slot.text()
	t := locale.T()
	c.file.SetText(path)
	if path == "" {
		c.file.SetText(t.Get("wm-tabs-font-built-in"))
	}
	if path != c.read {
		c.read, c.family = path, ""
		c.problem.SetVisible(false)
		if path != "" {
			async(c.k, func() readFamily {
				family, err := c.k.fileFamily(path)
				return readFamily{path, family, err}
			}, c.showFamily)
		}
	}
	c.picker.refresh()
}

func (c *wmFontEditor) showFamily(r readFamily) {
	if r.path != c.read {
		return
	}
	c.family = r.family
	c.showProblem(r.err)
	c.picker.refresh()
}

func (c *wmFontEditor) showProblem(err error) {
	c.problem.SetVisible(err != nil)
	if err != nil {
		c.problem.SetText(err.Error())
	}
}

func (c *wmFontEditor) pick(v any) error {
	family, ok := v.(string)
	if !ok {
		return fmt.Errorf("settings: a font family is a name, not %v", v)
	}
	async(c.k, func() resolvedFont {
		path, err := c.k.wmFontFile(family)
		return resolvedFont{family, path, err}
	}, func(r resolvedFont) {
		c.showProblem(r.err)
		if r.err == nil {
			_ = c.slot.set(r.path)
		}
	})
	return nil
}

func (k *kit) wmFontFile(family string) (string, error) {
	t := locale.T()
	path, err := k.fontFile(family)
	if err != nil {
		return "", fmt.Errorf("%s: %w", t.Get("wm-tabs-font-unresolved", i18n.Str("family", family)), err)
	}
	if _, err := k.fileFamily(path); err != nil {
		return "", fmt.Errorf("%s: %w", t.Get("wm-tabs-font-unreadable", i18n.Str("family", family)), err)
	}
	return path, nil
}

func readFontFamily(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the font file the config names, read to show its family
	if err != nil {
		return "", fmt.Errorf("settings: read the font %s: %w", path, err)
	}
	face, err := render.LoadFont(data)
	if err != nil {
		return "", fmt.Errorf("settings: read the font %s: %w", path, err)
	}
	return face.Family(), nil
}
