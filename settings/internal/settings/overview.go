package settings

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const overviewID = "overview"

const (
	overviewColumns   = 3
	overviewReadLimit = 5 * time.Second
	// overviewTileChars is every tile's description width, in characters,
	// so every tile asks one natural width and a line of tiles splits
	// evenly, at the same place on every line.
	overviewTileChars = 40
)

type tileStatus func(ctx context.Context, k *kit) (string, error)

var errNoStatus = errors.New("nothing to report")

var tileStatuses = map[string]tileStatus{
	"settings-nav-account":       accountStatus,
	"settings-nav-appearance":    appearanceStatus,
	"settings-nav-displays":      displaysStatus,
	"settings-nav-sound-section": soundStatus,
	"settings-nav-connectivity":  joinedStatus(networkStatus, bluetoothStatus),
	"settings-nav-power":         powerStatus,
}

func overviewPage() pageSpec {
	return pageSpec{
		id: overviewID, navKey: "settings-nav-overview", icon: "ld-layout-grid-symbolic",
		header: "settings-page-overview", lead: overviewTiles,
	}
}

func overviewTiles(k *kit) widget.Widget {
	t := locale.T()
	th := widget.Current()
	grid := widget.NewFlowBox(th.Spacing, th.Spacing)
	grid.AddClass("settings-overview")
	grid.SetMaxChildrenPerLine(overviewColumns)
	grid.SetJustify(widget.JustifyFill)
	grid.SetSelectionMode(widget.SelectionNone)
	heights := widget.NewSizeGroup(widget.SizeGroupVertical)
	for _, s := range k.nav {
		if s.key == "settings-nav-home" || len(s.pages) == 0 {
			continue
		}
		first := s.pages[0].id
		face := widget.NewBox(widget.Column, th.Spacing/2, 0)
		face.SetPadding(cardInsets())
		face.AppendAligned(k.icon(s.tileIcon(), "settings-overview-icon"), false, widget.AlignStart)
		face.Append(k.label(t.Get(s.key), "settings-overview-title", css.Heading), false)
		desc := k.note(t.Attr(s.key, "description"), "settings-overview-description")
		desc.SetWidthChars(overviewTileChars)
		desc.SetMaxWidthChars(overviewTileChars)
		face.Append(desc, false)
		if status, ok := k.statuses[s.key]; ok {
			showStatus(k, status, desc)
		}
		tile := k.roleButton(roleTile, face, func() { k.openPage(first) })
		tile.SetAccessibleName(t.Get(s.key))
		grid.Append(heights.Add(tile))
	}
	return grid
}

func showStatus(k *kit, status tileStatus, desc *widget.Label) {
	async(k, func() string {
		ctx, cancel := context.WithTimeout(context.Background(), overviewReadLimit)
		defer cancel()
		text, err := status(ctx, k)
		if err != nil {
			return ""
		}
		return text
	}, func(text string) {
		if text != "" {
			desc.SetText(text)
		}
	})
}

func accountStatus(_ context.Context, k *kit) (string, error) {
	db, err := k.readAccounts()
	if err != nil {
		return "", err
	}
	self, err := readSelf(db)
	if err != nil {
		return "", err
	}
	return locale.T().Get("settings-overview-signed-in", i18n.Str("name", self.DisplayName())), nil
}

func enumLabel(path, value string) string {
	meta, ok := config.Field(path)
	if !ok {
		return value
	}
	for i, v := range meta.Variants {
		if v == value {
			return enumLabels(meta)[i]
		}
	}
	return value
}

func appearanceStatus(_ context.Context, k *kit) (string, error) {
	st := k.store.svc.Config().Styling
	return locale.T().Get("settings-overview-appearance",
		i18n.Str("appearance", enumLabel("styling.appearance", string(st.Appearance))),
		i18n.Str("provider", enumLabel("styling.theme-provider", string(st.ColorExtractor.ThemeProvider)))), nil
}

func displaysStatus(context.Context, *kit) (string, error) {
	n := len(systemHost.outputs())
	if n == 0 {
		return "", errNoStatus
	}
	return locale.T().Get("settings-overview-displays", i18n.Int("count", int64(n))), nil
}

func soundStatus(ctx context.Context, _ *kit) (string, error) {
	svc, err := openPulse(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = svc.Close() }()
	out, err := svc.DefaultSink(ctx)
	if err != nil {
		return "", err
	}
	if out.Muted {
		return locale.T().Get("settings-overview-sound-muted", i18n.Str("device", out.Description)), nil
	}
	return locale.T().Get("settings-overview-sound", i18n.Str("device", out.Description), i18n.Int("percent", int64(math.Round(out.Volume.AveragePercentage())))), nil
}

func networkStatus(ctx context.Context, k *kit) (string, error) {
	ctl, err := k.network()
	if err != nil {
		return "", err
	}
	st, err := ctl.read(ctx)
	if err = errors.Join(err, ctl.close()); err != nil {
		return "", err
	}
	t := locale.T()
	switch {
	case st.active != "":
		return t.Get("settings-overview-wifi", i18n.Str("name", st.active)), nil
	case st.snap.Wired == network.StatusConnected:
		return t.Get("settings-overview-wired"), nil
	}
	return t.Get("settings-overview-offline"), nil
}

func powerStatus(ctx context.Context, k *kit) (string, error) {
	ctl, err := k.power()
	if err != nil {
		return "", err
	}
	st, err := ctl.read(ctx)
	if err = errors.Join(err, ctl.close()); err != nil {
		return "", err
	}
	if !st.hasBattery() {
		return "", errNoStatus
	}
	return locale.T().Get("settings-overview-battery", i18n.Str("charge", percentText(st.battery.Percentage)), i18n.Str("state", batteryDetail(st.battery))), nil
}

func bluetoothStatus(_ context.Context, k *kit) (string, error) {
	ctl, err := k.bluetooth()
	if err != nil {
		return "", err
	}
	st := ctl.state()
	if err := ctl.close(); err != nil {
		return "", err
	}
	t := locale.T()
	switch {
	case !st.Available:
		return "", errNoStatus
	case !st.Enabled:
		return t.Get("settings-overview-bluetooth-off"), nil
	}
	for _, d := range st.Devices {
		if d.Connected && d.DisplayName() != "" {
			return t.Get("settings-overview-bluetooth-connected", i18n.Str("name", d.DisplayName())), nil
		}
	}
	return t.Get("settings-overview-bluetooth-on"), nil
}

func joinedStatus(parts ...tileStatus) tileStatus {
	return func(ctx context.Context, k *kit) (string, error) {
		var texts []string
		var errs []error
		for _, part := range parts {
			text, err := part(ctx, k)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			texts = append(texts, text)
		}
		if len(texts) == 0 {
			return "", errors.Join(errs...)
		}
		return strings.Join(texts, " · "), nil
	}
}
