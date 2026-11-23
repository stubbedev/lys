package settings

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestARefusedCardWriteIsHeldAsADraftWithTheReason(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{}
	item := map[string]any{"above": int64(5)}
	refused := errors.New("the target refuses it")
	s := slot{get: func() any { return stored }, set: func(any) error { return refused }, unset: func() {}}
	c := newCardList(k, s, thresholdCards)
	c.local = append(c.local, item)
	c.commit()
	if !failedWith(c.draft, locale.T().Get("settings-list-draft"), refused.Error()) || !c.drafting {
		t.Fatalf("a refused write shows draft %v %q over %q", c.draft.Visible(), c.draft.Text(), c.draft.detail.Text())
	}
	accepted := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}
	c = newCardList(k, accepted, thresholdCards)
	c.local = append(c.local, item)
	c.commit()
	if c.draft.Visible() || c.drafting || !reflect.DeepEqual(stored, []any{item}) {
		t.Errorf("an accepted write shows draft %v and stored %v", c.draft.Visible(), stored)
	}
}

// Every card list wears the card form's classes unless its spec names
// its own: a list of its own cards, each card's header, body, rows and
// labels marked alike.
func TestACardListWearsTheCardFormClassesUnlessItNamesItsOwn(t *testing.T) {
	k := testKit(t, "")
	item := map[string]any{"above": int64(5)}
	var stored any = []any{item}
	s := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}
	plain := thresholdCards
	plain.classed = nil
	for name, tc := range map[string]struct {
		spec cardSpec
		want cardClasses
	}{
		"the card form's": {plain, cardFormClasses},
		"its own":         {thresholdCards, thresholdClasses},
	} {
		c := newCardList(k, s, tc.spec)
		for class, n := range map[string]int{tc.want.editor: 1, tc.want.list: 1, tc.want.card: 1, tc.want.header: 1, tc.want.body: 1, tc.want.row: len(tc.spec.fields), tc.want.label: len(tc.spec.fields)} {
			if got := len(treetest.WithClass(c, class)); got != n {
				t.Errorf("%s classes: %d widgets wear %q, want %d", name, got, class, n)
			}
		}
	}
	if cardFormClasses.row == thresholdClasses.row || cardFormClasses.card != thresholdClasses.card {
		t.Error("the threshold cards should differ from the card form in their rows alone")
	}
}
