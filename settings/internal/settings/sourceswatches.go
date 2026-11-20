package settings

import (
	"log"
	"reflect"
	"slices"
	"strconv"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/service/wallpaper/extract"
	"github.com/stubbedev/lys/desktop/styling/material"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var sourceColorRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newSourceSwatches(k, s) }
}

func themingWallpaper(w config.WallpaperConfig, theming string, outputs []string) (path string, cycling bool) {
	if w.CyclingDirectory != "" {
		return "", true
	}
	monitor := theming
	if monitor == "" && len(outputs) > 0 {
		monitor = slices.Min(outputs)
	}
	if m, ok := w.Monitor(monitor); ok && m.Wallpaper != "" {
		return m.Wallpaper, false
	}
	return w.Wallpaper, false
}

func connectedOutputs() []string {
	var out []string
	for _, ch := range systemLists["Monitor"]() {
		if ch.value != "" && ch.value != everyMonitor {
			out = append(out, ch.value)
		}
	}
	return out
}

type sourceOffer struct {
	path    string
	cycling bool
	colors  []material.ARGB
	failure string
}

func readSourceOffer(w config.WallpaperConfig, theming string) sourceOffer {
	path, cycling := themingWallpaper(w, theming, connectedOutputs())
	offer := sourceOffer{path: path, cycling: cycling}
	if path == "" {
		return offer
	}
	img, err := imagedecode.Open(path)
	if err != nil {
		offer.failure = locale.T().Get("settings-matugen-source-failed", i18n.Str("file", path), i18n.Str("error", err.Error()))
		return offer
	}
	offer.colors = extract.SourceColors(material.SampleImage(img, extract.SampleSide))
	return offer
}

type sourceInputs struct {
	wallpaper config.WallpaperConfig
	theming   string
}

type sourceSwatches struct {
	*widget.Box
	k       *kit
	slot    slot
	cards   *widget.Box
	note    *widget.Label
	problem *widget.Label
	shown   *choiceCards
	inputs  sourceInputs
	read    bool
	closed  bool
}

func newSourceSwatches(k *kit, s slot) *sourceSwatches {
	c := &sourceSwatches{Box: gapBox(widget.Column), k: k, slot: s}
	c.AddClass("source-swatches")
	c.cards = gapBox(widget.Column)
	c.note = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	c.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	c.Append(c.cards, false)
	c.Append(c.note, false)
	c.Append(c.problem, false)
	c.note.SetVisible(false)
	c.problem.SetVisible(false)
	c.refresh()
	return c
}

func (c *sourceSwatches) close() { c.closed = true }

func (c *sourceSwatches) refresh() {
	if c.shown != nil {
		c.shown.refresh()
	}
	cfg := c.k.store.svc.Config()
	inputs := sourceInputs{wallpaper: cfg.Wallpaper, theming: cfg.Styling.ColorExtractor.ThemingMonitor}
	if c.read && reflect.DeepEqual(inputs, c.inputs) {
		return
	}
	c.inputs, c.read = inputs, true
	async(c.k, func() sourceOffer { return readSourceOffer(inputs.wallpaper, inputs.theming) }, func(o sourceOffer) {
		if c.closed || !reflect.DeepEqual(inputs, c.inputs) {
			return
		}
		if o.failure != "" {
			log.Printf("settings: matugen source colours: %s", o.failure)
		}
		c.show(o)
	})
}

func (c *sourceSwatches) show(o sourceOffer) {
	t := locale.T()
	count := extract.SourceColorChoices
	if len(o.colors) > 0 {
		count = len(o.colors)
	}
	values := make([]string, count)
	labels := make([]string, count)
	pictures := make(map[string]picture, count)
	for i := range count {
		values[i] = strconv.Itoa(i)
		labels[i] = t.Get("settings-matugen-source-rank", i18n.Int("n", int64(i+1)))
		if i < len(o.colors) {
			fill := render.RGB(o.colors[i].Channels())
			labels[i] = o.colors[i].Hex()
			pictures[values[i]] = drawn(func(cv *render.Canvas, r render.Rect) { cv.RoundedRect(r, tileRadius/2, fill) })
		}
	}
	c.shown = newCards(c.k, c.indexSlot(count), values, labels, pictures)
	c.cards.Clear()
	c.cards.Append(c.shown, false)
	c.note.SetVisible(o.failure == "" && len(o.colors) == 0)
	switch {
	case o.cycling:
		c.note.SetText(t.Get("settings-matugen-source-cycling"))
	case o.path == "":
		c.note.SetText(t.Get("settings-matugen-source-none"))
	}
	c.problem.SetText(o.failure)
	c.problem.SetVisible(o.failure != "")
}

func (c *sourceSwatches) indexSlot(count int) slot {
	return slot{
		get: func() any {
			stored := c.k.store.svc.Config().Styling.ColorExtractor.MatugenSourceColor
			return strconv.Itoa(extract.SourceColorIndex(stored, count))
		},
		set: func(v any) error {
			s, _ := v.(string)
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				log.Panicf("settings: the source colour card %q is no index: %v", s, err)
			}
			return c.slot.set(i)
		},
		unset: c.slot.unset,
	}
}
