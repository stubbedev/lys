package lockview

import (
	"strconv"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
)

const (
	mediaArtPx     = 48
	mediaControlPx = 20
	batteryIconPx  = 16
	playIcon       = "ld-circle-play-symbolic"
	pauseIcon      = "ld-circle-pause-symbolic"
	nextIcon       = "ld-skip-forward-symbolic"
)

// MediaIcons are the theme icons the media row draws.
func MediaIcons() []string { return []string{playIcon, pauseIcon, nextIcon} }

// Track is what the media row shows of the playing track: ArtPath is a
// local cover image, empty for none.
type Track struct {
	Title, Artist, ArtPath string
	Playing, CanNext       bool
}

// MediaRow is the lock screen's row under the clock: the cover, the
// title over the artist, play or pause and next.
type MediaRow struct {
	Root          *widget.Box
	Title, Artist *widget.Label
	Play, Next    *widget.Button
	art           *widget.Image
	artPath       string
	playIcon      *widget.Icon
}

// NewMediaRow is a hidden media row whose buttons run onPlayPause and
// onNext.
func NewMediaRow(font render.Font, onPlayPause, onNext func()) *MediaRow {
	th := widget.Current()
	px := widgets.TextPx(styling.Body, 1)
	m := &MediaRow{
		art:      widget.NewFileImage(""),
		Title:    widget.NewLabel(font, px, "", 0),
		Artist:   widget.NewLabel(font, px, "", 0),
		playIcon: widget.NewThemeIcon(playIcon, mediaControlPx),
	}
	m.art.AddClass("lock-media-art")
	m.art.SetScale(widget.ImageCover)
	m.art.SetSizeRequest(mediaArtPx, mediaArtPx)
	m.art.SetMaximumSize(mediaArtPx, mediaArtPx)
	m.Title.AddClass("lock-media-title", css.Heading)
	m.Artist.AddClass("lock-media-artist", css.DimLabel)
	m.Play = mediaControl(m.playIcon, onPlayPause)
	m.Next = mediaControl(widget.NewThemeIcon(nextIcon, mediaControlPx), onNext)
	nameControl(m.Next, i18n.T("lock-media-next"))
	track := widget.NewBox(widget.Column, 0, 0)
	track.Append(m.Title, false).Append(m.Artist, false)
	m.Root = widget.NewBox(widget.Row, th.Spacing, 0)
	m.Root.AddClass("lock-media")
	m.Root.AppendAligned(m.art, false, widget.AlignCenter)
	m.Root.AppendAligned(track, false, widget.AlignCenter)
	m.Root.AppendAligned(m.Play, false, widget.AlignCenter)
	m.Root.AppendAligned(m.Next, false, widget.AlignCenter)
	m.Root.SetVisible(false)
	return m
}

func mediaControl(icon *widget.Icon, onClick func()) *widget.Button {
	b := widget.NewButton(icon, 0, 0)
	b.AddClass("lock-media-control", css.Ghost, css.Icon)
	b.SetCursorName("pointer")
	b.OnClick = onClick
	return b
}

func nameControl(b *widget.Button, name string) {
	b.SetAccessibleName(name)
	b.SetTooltip(name)
}

// Set shows t, or hides the row when nothing plays.
func (m *MediaRow) Set(t Track, playing bool) {
	m.Root.SetVisible(playing)
	if !playing {
		return
	}
	m.Title.SetText(t.Title)
	m.Artist.SetText(t.Artist)
	m.Artist.SetVisible(t.Artist != "")
	if t.ArtPath != m.artPath {
		m.artPath = t.ArtPath
		m.art.SetFile(t.ArtPath)
	}
	m.art.SetVisible(t.ArtPath != "")
	icon, name := playIcon, i18n.T("lock-media-play")
	if t.Playing {
		icon, name = pauseIcon, i18n.T("lock-media-pause")
	}
	m.playIcon.SetThemeName(icon)
	nameControl(m.Play, name)
	m.Next.SetVisible(t.CanNext)
}

// BatteryLevel is the battery's charge in whole percent.
type BatteryLevel struct {
	Percent  int
	Charging bool
}

// BatteryChip is the lock screen's corner chip: the charging icon while
// charging, beside the charge.
type BatteryChip struct {
	Root     *widget.Box
	Charging *widget.Icon
	Label    *widget.Label
}

// NewBatteryChip is a hidden chip showing chargingIcon while charging.
func NewBatteryChip(font render.Font, chargingIcon string) *BatteryChip {
	th := widget.Current()
	b := &BatteryChip{
		Charging: widget.NewThemeIcon(chargingIcon, batteryIconPx),
		Label:    widget.NewLabel(font, widgets.TextPx(styling.Body, 1), "", 0),
	}
	b.Charging.SetAccessibleName(i18n.T("lock-battery-charging"))
	b.Label.AddClass(css.Numeric)
	b.Root = widget.NewBox(widget.Row, th.Spacing/2, 0)
	b.Root.AddClass("lock-battery")
	b.Root.AppendAligned(b.Charging, false, widget.AlignCenter)
	b.Root.AppendAligned(b.Label, false, widget.AlignCenter)
	b.Root.SetVisible(false)
	return b
}

// Set shows l, or hides the chip on a machine without a battery.
func (b *BatteryChip) Set(l BatteryLevel, present bool) {
	b.Root.SetVisible(present)
	if !present {
		return
	}
	b.Label.SetText(i18n.T("lock-battery", i18n.Str("percent", strconv.Itoa(l.Percent))))
	b.Charging.SetVisible(l.Charging)
}

// SwitchUserButton is the lock screen's footer offering another account.
func SwitchUserButton(font render.Font, onClick func()) *widget.Button {
	label := widget.NewLabel(font, widgets.TextPx(styling.Body, 1), i18n.T("lock-switch-user"), 0)
	b := widget.NewButton(label, 0, 0)
	b.AddClass("lock-switch-user", css.Pill, css.Ghost)
	b.SetCursorName("pointer")
	b.OnClick = onClick
	return b
}
