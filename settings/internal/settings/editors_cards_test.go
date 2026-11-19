package settings

import (
	"errors"
	"reflect"
	"strings"
	"testing"
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
	if !c.draft.Visible() || !strings.Contains(c.draft.Text(), refused.Error()) || !c.drafting {
		t.Fatalf("a refused write shows draft %v %q", c.draft.Visible(), c.draft.Text())
	}
	accepted := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}
	c = newCardList(k, accepted, thresholdCards)
	c.local = append(c.local, item)
	c.commit()
	if c.draft.Visible() || c.drafting || !reflect.DeepEqual(stored, []any{item}) {
		t.Errorf("an accepted write shows draft %v and stored %v", c.draft.Visible(), stored)
	}
}
