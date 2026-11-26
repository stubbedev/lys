package settings

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestTheSharePickerOpensOnAPagePickedFromPictures(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, pageRow(t, "privacy", "share-picker.default-page")).(*choiceCards)
	if !ok {
		t.Fatal("the share picker's first page is not picked from pictures")
	}
	seen := map[string]string{}
	for value, p := range sharePickerPagePictures {
		px := string(paintPicture(t, p))
		if other, dup := seen[px]; dup {
			t.Errorf("%s and %s draw the same picture", value, other)
		}
		seen[px] = value
	}
	i := slices.IndexFunc(c.cards, func(card *choiceCard) bool { return card.value == string(config.SharePickerRegion) })
	c.cards[i].PressAt(widget.Point{})
	if v := k.store.value("share-picker.default-page"); v != "region" {
		t.Errorf("pressing Region stored %v", v)
	}
}

func TestOfferingToRememberIsTheRestoreCheckboxNotHidden(t *testing.T) {
	k := testKit(t, "")
	spec := pageRow(t, "privacy", "share-picker.hide-token-restore")
	if spec.labelKey() != "settings-share-picker-offer-restore" {
		t.Errorf("the row reads %s", spec.labelKey())
	}
	c, ok := fieldControl(k, k.store, spec).(*toggle)
	if !ok {
		t.Fatalf("the row is a %T, want a switch", fieldControl(k, k.store, spec))
	}
	if !c.On() {
		t.Fatal("with the checkbox shown, offering to remember is off")
	}
	c.Toggle()
	if v := k.store.value("share-picker.hide-token-restore"); v != true {
		t.Errorf("turning the offer off stored hide-token-restore %v, want true", v)
	}
	k2 := testKit(t, "share-picker:\n  hide-token-restore: true\n")
	if c := fieldControl(k2, k2.store, spec).(*toggle); c.On() {
		t.Error("with the checkbox hidden, offering to remember is on")
	}
}

func TestThePreviewResolutionIsPickedFromVideoHeights(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, pageRow(t, "privacy", "share-picker.resize-size"))
	got := chipLabels(p)
	for i := range got {
		got[i] = unisolated(got[i])
	}
	if !slices.Equal(got, []string{"360p", "480p", "720p", "1080p"}) {
		t.Fatalf("the chips read %q", got)
	}
	if p.active() != len(p.presets) || !p.typed.Visible() {
		t.Errorf("the default 640 shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	pressChip(p.ChoiceChips, 2)
	if v := k.store.value("share-picker.resize-size"); v != int64(720) {
		t.Errorf("choosing 720p stored %v", v)
	}
}

func TestThePickersGeometryIsUnderMoreOptions(t *testing.T) {
	p, _ := pageByID(layout(config.Defaults(), bothParts), "privacy")
	for _, s := range p.sections {
		for _, r := range s.flatRows() {
			switch r.path {
			case "share-picker.width", "share-picker.height", "share-picker.widget-size", "share-picker.resize-size":
				if !s.advanced {
					t.Errorf("%s shows outside More Options", r.path)
				}
			case "share-picker.default-page", "share-picker.hide-token-restore":
				if s.advanced {
					t.Errorf("%s is folded under More Options", r.path)
				}
			}
		}
	}
}

func sharePickerTryButton(t *testing.T, k *kit) *tryButton {
	t.Helper()
	tries := treetest.All[*tryButton](buildPage(k, privacyPage(nil)))
	if len(tries) != 1 {
		t.Fatalf("the privacy page has %d try buttons, want the share picker's preview", len(tries))
	}
	return tries[0]
}

func TestPreviewShowsTheSharePickerThroughLysShell(t *testing.T) {
	k := testKit(t, "")
	var ran [][]string
	k.shellCommand = func(args ...string) error {
		ran = append(ran, args)
		return nil
	}
	b := sharePickerTryButton(t, k)
	b.button.OnClick()
	if len(ran) != 1 || !slices.Equal(ran[0], []string{"portal", "show", "screen-cast-preview"}) {
		t.Fatalf("Preview ran lys-shell %q, want portal show screen-cast-preview", ran)
	}
	if b.outcome.Visible() {
		t.Errorf("a shown preview reports %q", b.outcome.Text())
	}
}

func TestAPreviewLysShellRefusesSaysWhy(t *testing.T) {
	k := testKit(t, "")
	k.shellCommand = func(...string) error { return errors.New("SharePicker service not running") }
	b := sharePickerTryButton(t, k)
	b.button.OnClick()
	if !failedWith(b.outcome, locale.T().Get("settings-try-failed"), "SharePicker service not running") {
		t.Errorf("a refused preview shows %q over %q (visible %v)", b.outcome.Text(), b.outcome.detail.Text(), b.outcome.Visible())
	}
}

// The preview button floated between the section's heading and its card.
// It is the card's first row, its title and description saying what it
// opens, ahead of the picker's settings.
func TestTheSharePickerPreviewIsItsCardsFirstRow(t *testing.T) {
	win := laidOutPage(t, "", "privacy", 1664)
	tries := treetest.All[*tryButton](win.page)
	if len(tries) != 1 {
		t.Fatalf("%d try buttons on the privacy page", len(tries))
	}
	var card *widget.ItemGroup
	for _, g := range treetest.All[*widget.ItemGroup](win.page) {
		if treetest.Contains(g, tries[0]) {
			card = g
		}
	}
	if card == nil {
		t.Fatal("the preview sits in no card")
	}
	rows := treetest.All[*fieldRow](card)
	if len(rows) == 0 {
		t.Fatal("the preview's card holds no setting")
	}
	if try, first := tries[0].Bounds(), rows[0].Bounds(); try.Y >= first.Y {
		t.Errorf("the preview at y=%d comes after the card's first setting at y=%d", try.Y, first.Y)
	}
}
