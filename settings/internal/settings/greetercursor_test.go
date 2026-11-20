package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestTheGreeterMatchesMyCursorUntilItIsGivenItsOwn(t *testing.T) {
	k := testKit(t, "")
	p := layoutPage(t, k, "lock")
	match := rowOf(t, p, "login.greeter.match-cursor")
	theme, size := rowOf(t, p, "login.greeter.cursor-theme"), rowOf(t, p, "login.greeter.cursor-size")
	if !match.Visible() {
		t.Fatal("Match my cursor is hidden")
	}
	if theme.Visible() || size.Visible() {
		t.Fatalf("matching my cursor, the theme shows %v and the size %v", theme.Visible(), size.Visible())
	}
	setRow(t, p, "login.greeter.match-cursor", false)
	if !theme.Visible() || !size.Visible() {
		t.Errorf("with a cursor of its own, the theme shows %v and the size %v", theme.Visible(), size.Visible())
	}
}

func TestTheLoginScreenIsToldWhetherToMatchMyCursor(t *testing.T) {
	for _, match := range []bool{true, false} {
		k := testKit(t, "")
		if err := k.store.set("login.greeter.match-cursor", match); err != nil {
			t.Fatal(err)
		}
		body, err := greeterDocument(k.store)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := config.ParseYAML(body)
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := config.PathValue(doc, "login.greeter.match-cursor"); !ok || v != match {
			t.Errorf("match-cursor %v staged %v (%v):\n%s", match, v, ok, body)
		}
	}
}
