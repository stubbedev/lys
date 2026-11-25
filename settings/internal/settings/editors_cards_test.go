package settings

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

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

// A clock calendar's folder field, a field of its own width, stood packed
// at the header's start with some 390px empty before the trash, while
// the card's Color sits in the control column. An identity that does not
// fill sits at the header's end as a field below sits at its row's; one
// that fills, as an account's name, still takes the header's width.
func TestACardsIdentitySitsWhereItsFieldsWould(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    calendars:\n      - path: /tmp/calendar\n  mail:\n    accounts:\n      - name: Work\n        query: tag:inbox\n")
	for _, tc := range []struct{ page, path string }{
		{"clock", "modules.clock.calendars"},
		{"mail", "modules.mail.accounts"},
	} {
		c := fieldControl(k, k.store, pageRow(t, tc.page, tc.path)).(*cardList)
		themed(k, c)
		if len(c.cards) != 1 {
			t.Fatalf("%s: %d cards", tc.path, len(c.cards))
		}
		end := 0
		treetest.Walk(c.cards[0].identity, func(w widget.Widget) {
			r, ok := w.(interface{ Role() widget.Role })
			if ok && (r.Role() == widget.RoleEntry || r.Role() == widget.RoleButton) {
				b := w.(widget.Boundser).Bounds()
				end = max(end, b.X+b.W)
			}
		})
		trash := treetest.Button(t, c.cards[0].root, locale.T().Get("settings-list-remove")).Bounds()
		if gap := trash.X - end; gap > choiceCardGap {
			t.Errorf("%s: the identity ends %dpx before the trash", tc.path, gap)
		}
	}
}
