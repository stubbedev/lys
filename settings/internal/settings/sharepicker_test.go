package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

func TestTheSharePickerOpensOnAPagePickedFromPictures(t *testing.T) {
	k := testKit(t, "")
	c, ok := fieldControl(k, k.store, pageRow(t, "privacy", "share-picker.default-page")).(*choiceCards)
	if !ok {
		t.Fatal("the share picker's first page is not picked from pictures")
	}
	seen := map[string]string{}
	for value, p := range sharePickerPagePictures {
		px := string(paintPicture(p))
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
	p.chips[2].SetActive(true)
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
