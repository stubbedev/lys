package settings

import "testing"

func TestTheWebEngineIsPickedByName(t *testing.T) {
	k := testKit(t, "")
	e := fieldControl(k, k.store, field(webEnginePath, webEngineRow)).(*webEngineEditor)
	if e.pick.Selected() != 0 || e.custom.Visible() {
		t.Fatalf("the default engine shows as %d, custom %v", e.pick.Selected(), e.custom.Visible())
	}
	e.pick.OnSelect(1)
	if n, u := k.store.value(webEnginePath), k.store.value(webURLPath); n != webEngines[1].name || u != webEngines[1].url {
		t.Fatalf("picking %s stored %v %v", webEngines[1].name, n, u)
	}
	if e.custom.Visible() {
		t.Error("a listed engine shows the custom fields")
	}
	e.pick.OnSelect(len(webEngines))
	if !e.custom.Visible() {
		t.Fatal("Custom does not show the name and address")
	}
	if n := k.store.value(webEnginePath); n != webEngines[1].name {
		t.Errorf("choosing Custom rewrote the engine to %v", n)
	}
	e.inputs[0].SetText("Searx")
	e.inputs[1].SetText("https://searx.example/?q={query}")
	if n, u := k.store.value(webEnginePath), k.store.value(webURLPath); n != "Searx" || u != "https://searx.example/?q={query}" {
		t.Fatalf("the custom engine stored %v %v", n, u)
	}
	k2 := testKit(t, "launcher:\n  search:\n    web-engine: Mine\n    web-url: \"https://mine/?q={query}\"\n")
	e2 := fieldControl(k2, k2.store, field(webEnginePath, webEngineRow)).(*webEngineEditor)
	if e2.pick.Selected() != len(webEngines) || !e2.custom.Visible() {
		t.Errorf("an unlisted engine shows as %d, custom %v", e2.pick.Selected(), e2.custom.Visible())
	}
}
