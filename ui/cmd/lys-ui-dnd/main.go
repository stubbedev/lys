// Command lys-ui-dnd is the cross-window drag-and-drop demo: two
// toplevel windows, each holding color swatches that are both drag
// sources and drop targets. Dragging a swatch over a window of the
// other one carries the color through the compositor's data device -
// the wl_data_offer path real cross-application drops take, exercised
// between two windows of one process. A click cycles the swatch's own
// color; Escape quits.
package main

import (
	"errors"
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"

	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
)

type swatch struct {
	*widget.Button
	col render.Color
}

func newSwatch(col render.Color) *swatch {
	s := &swatch{col: col}
	s.Button = widget.NewButton(widget.NewSpacer(48, 48), 0, 0)
	s.SetColor(col)
	s.OnClick = func() { s.SetColor(nextColor(s.col)) }
	return s
}

func (s *swatch) SetColor(c render.Color) {
	s.col = c
	s.Bg = c
	s.Invalidate()
}

func (s *swatch) hex() string {
	return render.FormatHex(s.col)
}

func (s *swatch) DragContent() *widget.DragContent {
	return &widget.DragContent{Content: transfer.Text(s.hex())}
}

func (s *swatch) DragEnter(mimes []string, _ widget.Point) string {
	if slices.Contains(mimes, transfer.MimeText) {
		return transfer.MimeText
	}
	return ""
}

func (s *swatch) Drop(mime string, data []byte, _ widget.Point) {
	if mime != transfer.MimeText {
		return
	}
	if c, ok := render.ParseHex(strings.TrimSpace(string(data))); ok {
		s.SetColor(c)
	}
}

func (s *swatch) HitTest(p widget.Point) widget.Widget {
	if s.Bounds().Contains(p.X, p.Y) {
		return s
	}
	return nil
}

func main() {
	if err := run(); err != nil && !errors.Is(err, app.ErrClosed) {
		log.Fatal(err)
	}
}

func run() error {
	sess, err := app.Connect()
	if err != nil {
		return err
	}
	defer sess.Close()
	face, err := app.UIFont(14)
	if err != nil {
		return err
	}
	application := app.NewApplication(sess)
	defer application.Close()
	theme := widget.Current()

	for i, title := range []string{"lys-ui dnd source", "lys-ui dnd target"} {
		hint := widget.NewLabel(face, 12, "drag a swatch onto the other window; click cycles; Escape quits", theme.TextMuted)
		row := widget.NewBox(widget.Row, 12, 12)
		for _, col := range palette[i] {
			row.Append(newSwatch(col), false)
		}
		root := widget.NewBox(widget.Column, 14, 16)
		root.Append(hint, false)
		root.Append(row, false)
		if _, err := application.NewWindow(app.WindowConfig{
			Title:      title,
			AppID:      "dev.stubbe.lys.ui.dnd",
			Width:      360,
			Height:     200,
			Root:       root,
			Background: theme.Bg,
		}); err != nil {
			return err
		}
	}
	application.OnKey(func(_ *widget.Router, code uint32, mods app.Mods) {
		if mods&app.ModAlt == 0 && sess.KeySym(code) == xkb.KeyEscape {
			application.Quit()
		}
	})
	return application.Run()
}

var palette = [2][3]render.Color{
	{render.RGB(0x89, 0xB4, 0xFA), render.RGB(0xF9, 0xE2, 0xAF), render.RGB(0xA6, 0xE3, 0xA1)},
	{render.RGB(0xF5, 0xC2, 0xE7), render.RGB(0x94, 0xE2, 0xD5), render.RGB(0xF3, 0x8B, 0xA8)},
}

func nextColor(c render.Color) render.Color {
	return render.RGB(c.R()^0x55, c.G(), c.B())
}
