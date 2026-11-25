package widget

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

type framework int

const (
	next framework = iota
	svelte
	nuxt
)

func newTestCombobox(t *testing.T) (*Combobox[framework], *fakeSurfaceHost) {
	t.Helper()
	c := NewCombobox(goldenFace(t), "Select framework...",
		ComboboxOption[framework]{Value: next, Label: "Next.js"},
		ComboboxOption[framework]{Value: svelte, Label: "SvelteKit"},
		ComboboxOption[framework]{Value: nuxt, Label: "Nuxt.js"},
	)
	host := &fakeSurfaceHost{}
	SetSurfaceHost(c, host)
	arrangeTree(t, c, 300, 40)
	return c, host
}

// pickLabel opens the combobox and picks the option labeled label
// through its command.
func pickLabel(t *testing.T, c *Combobox[framework], host *fakeSurfaceHost, label string) {
	t.Helper()
	c.trigger.OnClick()
	cmd := host.popovers[len(host.popovers)-1].(*Box).Children()[0].(*Command)
	cmd.SetQuery(label)
	it, ok := cmd.Highlighted()
	if !ok || it.Label != label {
		t.Fatalf("searching %q highlights %q", label, it.Label)
	}
	it.OnSelect()
}

// A single-choice combobox shows its pick, closes, and clears when the
// selected option is picked again.
func TestComboboxSingle(t *testing.T) {
	c, host := newTestCombobox(t)
	var changes [][]framework
	c.OnChanged = func(s []framework) { changes = append(changes, s) }
	if c.text.Text() != "Select framework..." {
		t.Fatalf("trigger reads %q, want the placeholder", c.text.Text())
	}
	pickLabel(t, c, host, "SvelteKit")
	if !slices.Equal(c.Selected(), []framework{svelte}) || c.text.Text() != "SvelteKit" || host.closed != 1 {
		t.Fatalf("selected %v reading %q, %d closes", c.Selected(), c.text.Text(), host.closed)
	}
	pickLabel(t, c, host, "SvelteKit")
	if len(c.Selected()) != 0 || len(changes) != 2 {
		t.Errorf("picking the selected option left %v after %d changes", c.Selected(), len(changes))
	}
}

// A multiple-choice combobox toggles picks, keeps option order, and
// stays open.
func TestComboboxMultiple(t *testing.T) {
	c, host := newTestCombobox(t)
	c.SetMultiple(true)
	pickLabel(t, c, host, "Nuxt.js")
	pickLabel(t, c, host, "Next.js")
	if !slices.Equal(c.Selected(), []framework{next, nuxt}) || c.text.Text() != "Next.js, Nuxt.js" || host.closed != 0 {
		t.Errorf("selected %v reading %q, %d closes", c.Selected(), c.text.Text(), host.closed)
	}
}

func TestComboboxRejectsUnknownValues(t *testing.T) {
	c, _ := newTestCombobox(t)
	defer func() {
		if recover() == nil {
			t.Error("an unknown value did not panic")
		}
	}()
	c.SetSelected(framework(9))
}

// TestGoldenCombobox pins the trigger empty and with a pick, and
// the open list with its check, in both palettes.
func TestGoldenCombobox(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *Theme
	}{{"dark", DarkTheme()}, {"light", LightTheme()}} {
		t.Run(tc.name, func(t *testing.T) {
			SetTheme(tc.th)
			defer SetTheme(nil)
			empty, _ := newTestCombobox(t)
			picked, host := newTestCombobox(t)
			picked.SetSelected(svelte)
			picked.trigger.OnClick()
			col := NewBox(Column, 12, 0)
			col.Append(empty, false).Append(picked, false).Append(host.popovers[0], false)
			NewGolden(t, col, "combobox-"+tc.name, goldenTheme(tc.th), goldenFrame(220, 260))
		})
	}
}

// An option's check trails its row, after the label, and the trigger
// reads its placeholder in the button's own color: the check sat in a
// leading slot that indented every option, and the placeholder was
// muted (#281).
func TestComboboxCheckTrailsTheOption(t *testing.T) {
	c, host := newTestCombobox(t)
	c.trigger.OnClick()
	content := host.popovers[len(host.popovers)-1]
	SetStyleAnchor(content, c)
	arrangeTree(t, content, 300, 300)
	cmd := content.(*Box).Children()[0].(*Command)
	row := cmd.rows[0].Children()
	if row[len(row)-1] != Widget(c.list.checks[0]) {
		t.Errorf("the first option's row ends with %T, want its check", row[len(row)-1])
	}
	if _, ok := row[0].(*Label); !ok {
		t.Errorf("the first option's row starts with %T, want its label", row[0])
	}
	if got, muted := c.text.textColor(c.text.style(c.text)), Current().DisabledText(); got == muted {
		t.Errorf("the placeholder reads muted %#x, want the trigger's color", uint32(got))
	}
}

func newTestComboboxEntry(t *testing.T, options ...ComboboxOption[string]) (*ComboboxEntry, *fakeSurfaceHost) {
	t.Helper()
	c := NewComboboxEntry(goldenFace(t), 14, options...)
	host := &fakeSurfaceHost{}
	SetSurfaceHost(c, host)
	arrangeTree(t, c, 300, 40)
	return c, host
}

// openedCommand is the Command of the list host last opened.
func openedCommand(host *fakeSurfaceHost) *Command {
	return host.popovers[len(host.popovers)-1].(*Box).Children()[0].(*Command)
}

// A combobox entry is one control: its chevron sits inside the entry,
// which is its only focus stop, and a press on the chevron opens the
// list; typing edits the text without opening it, and picking an option
// shows it, closes the list and reports it.
func TestComboboxEntryOpensFromItsChevronAndPicks(t *testing.T) {
	c, host := newTestComboboxEntry(t, ComboboxOption[string]{Value: "dk", Label: "Danish"}, ComboboxOption[string]{Value: "us", Label: "English (US)"})
	var picked []string
	c.OnPick = func(v string) { picked = append(picked, v) }
	focusable := 0
	walkWidgets(c, func(w Widget) bool {
		if Describe(w).Focusable {
			focusable++
		}
		_, button := w.(*Button)
		if button {
			t.Errorf("the combobox holds a button %T beside its entry", w)
		}
		return true
	})
	if focusable != 1 || RoleOf(c) != RoleComboBox {
		t.Fatalf("%d focus stops, role %v; want the entry alone, a combobox", focusable, RoleOf(c))
	}
	c.entry.Insert("d")
	if len(host.popovers) != 0 {
		t.Fatal("typing opened the list")
	}
	r := c.entry.trailingRect()
	c.entry.ClickAt(Point{X: r.X + r.W/2, Y: r.Y + r.H/2})
	if len(host.popovers) != 1 || !c.Opened() {
		t.Fatalf("a press on the chevron opened %d lists", len(host.popovers))
	}
	cmd := openedCommand(host)
	cmd.SetQuery("English")
	it, ok := cmd.Highlighted()
	if !ok || it.Label != "English (US)" {
		t.Fatalf("searching highlights %q", it.Label)
	}
	it.OnSelect()
	if c.entry.Text() != "us" || host.closed != 1 || c.Opened() || len(picked) != 1 || picked[0] != "us" {
		t.Fatalf("picking left %q, %d closes, picks %q", c.entry.Text(), host.closed, picked)
	}
	c.entry.KeyAction(KeyDown, 0)
	if len(host.popovers) != 2 {
		t.Fatal("Down in the entry did not open the list")
	}
	if !commandChecked(t, openedCommand(host), "English (US)") || commandChecked(t, openedCommand(host), "Danish") {
		t.Error("the check does not trail the option the text holds alone")
	}
}

// commandChecked reports whether the row labeled label shows its check.
func commandChecked(t *testing.T, cmd *Command, label string) bool {
	t.Helper()
	for _, r := range cmd.rows {
		if r.item.Label != label {
			continue
		}
		on := false
		walkWidgets(r.item.Trailing, func(w Widget) bool {
			if s, ok := w.(*Symbol); ok && s.kind == SymbolCheck {
				on = true
			}
			return true
		})
		return on
	}
	t.Fatalf("no row %q", label)
	return false
}

// Options with a group list under its heading, in the order the groups
// first appear, and an option's detail trails its row and is searched.
func TestComboboxGroupsAndDetails(t *testing.T) {
	c, host := newTestComboboxEntry(t,
		ComboboxOption[string]{Value: "caps:escape", Label: "Caps Lock is Escape", Group: "Caps Lock behavior"},
		ComboboxOption[string]{Value: "grp:alt_shift_toggle", Label: "Alt+Shift", Group: "Switching layout"},
		ComboboxOption[string]{Value: "caps:none", Label: "Caps Lock is disabled", Group: "Caps Lock behavior"},
		ComboboxOption[string]{Value: "systemctl suspend", Label: "Suspend", Detail: "systemctl suspend"},
	)
	c.Open()
	cmd := openedCommand(host)
	var headings []string
	for _, h := range cmd.headings {
		headings = append(headings, h.Text())
	}
	if !slices.Equal(headings, []string{"Caps Lock behavior", "Switching layout", ""}) {
		t.Fatalf("headings %q", headings)
	}
	if cmd.rows[1].item.Label != "Caps Lock is disabled" || cmd.rows[1].group != 0 {
		t.Errorf("the second option of a group is not listed under it: %q in group %d", cmd.rows[1].item.Label, cmd.rows[1].group)
	}
	last := cmd.rows[len(cmd.rows)-1].item
	if got := treeLabels(last.Trailing); !slices.Equal(got, []string{"systemctl suspend"}) {
		t.Errorf("the detail row trails %q", got)
	}
	cmd.SetQuery("systemctl")
	if it, ok := cmd.Highlighted(); !ok || it.Label != "Suspend" {
		t.Errorf("searching the detail highlights %q", it.Label)
	}
	c.Open()
	cmd = openedCommand(host)
	cmd.SetQuery("Alt+Shift")
	if it, ok := cmd.Highlighted(); !ok || it.Label != "Alt+Shift" {
		t.Errorf("a grouped option is not found by its label: %q", it.Label)
	}
}

// An unheaded run stays where it stands: options without a group after a
// headed one list after it, not gathered into the unheaded run before it.
func TestComboboxUnheadedRunsStayInPlace(t *testing.T) {
	c, host := newTestComboboxEntry(t,
		ComboboxOption[string]{Value: "auto", Label: "Automatic"},
		ComboboxOption[string]{Value: "gone", Label: "gone", Group: "Not found"},
		ComboboxOption[string]{Value: "a", Label: "Alpha"},
		ComboboxOption[string]{Value: "b", Label: "Beta"},
		ComboboxOption[string]{Value: "lost", Label: "lost", Group: "Not found"},
	)
	c.Open()
	cmd := openedCommand(host)
	var headings []string
	for _, h := range cmd.headings {
		headings = append(headings, h.Text())
	}
	if !slices.Equal(headings, []string{"", "Not found", ""}) {
		t.Fatalf("headings %q", headings)
	}
	var rows []string
	var groups []int
	for _, r := range cmd.rows {
		rows, groups = append(rows, r.item.Label), append(groups, r.group)
	}
	if !slices.Equal(rows, []string{"Automatic", "gone", "lost", "Alpha", "Beta"}) || !slices.Equal(groups, []int{0, 1, 1, 2, 2}) {
		t.Errorf("the rows list %q in groups %v", rows, groups)
	}
}

// treeLabels are the texts of the labels under w.
func treeLabels(w Widget) []string {
	var out []string
	walkWidgets(w, func(x Widget) bool {
		if l, ok := x.(*Label); ok {
			out = append(out, l.Text())
		}
		return true
	})
	return out
}

// The empty value reads as the placeholder and is listed first; picking
// it empties the entry; taking it back drops both.
func TestComboboxEntryEmptyValue(t *testing.T) {
	c, host := newTestComboboxEntry(t, ComboboxOption[string]{Value: "DP-1", Label: "DP-1"})
	c.SetEmpty("Default")
	if c.entry.Placeholder() != "Default" {
		t.Fatalf("placeholder %q", c.entry.Placeholder())
	}
	c.entry.SetText("DP-1")
	c.Open()
	cmd := openedCommand(host)
	if cmd.rows[0].item.Label != "Default" || !commandChecked(t, cmd, "DP-1") || commandChecked(t, cmd, "Default") {
		t.Fatalf("the list starts with %q", cmd.rows[0].item.Label)
	}
	var picked []string
	c.OnPick = func(v string) { picked = append(picked, v) }
	cmd.rows[0].item.OnSelect()
	if c.entry.Text() != "" || !slices.Equal(picked, []string{""}) {
		t.Fatalf("picking the empty value left %q, picks %q", c.entry.Text(), picked)
	}
	c.Open()
	if !commandChecked(t, openedCommand(host), "Default") {
		t.Error("the empty text does not check the empty value")
	}
	c.SetEmpty("")
	if c.entry.Placeholder() != "" || len(c.Options()) != 1 {
		t.Errorf("taking the empty value back left placeholder %q, %d options", c.entry.Placeholder(), len(c.Options()))
	}
}

// An action follows the entry inside the combobox, and is taken out
// again by nil.
func TestComboboxEntryTrailingAction(t *testing.T) {
	c, _ := newTestComboboxEntry(t)
	browse := NewButton(NewSymbol(SymbolSearch, 16), 0, 0)
	c.SetAction(browse)
	kids := c.row.Children()
	if len(kids) != 2 || kids[0] != Widget(c.entry) || kids[1] != Widget(browse) || c.Action() != Widget(browse) {
		t.Fatalf("the row holds %v, want the entry then the action", kids)
	}
	arrangeTree(t, c, 300, 40)
	if browse.Bounds().X < c.entry.Bounds().X+c.entry.Bounds().W {
		t.Errorf("the action at x %d does not follow the entry ending at %d", browse.Bounds().X, c.entry.Bounds().X+c.entry.Bounds().W)
	}
	c.SetAction(nil)
	if len(c.row.Children()) != 1 || c.Action() != nil {
		t.Errorf("removing the action left %d widgets", len(c.row.Children()))
	}
}

// A disabled combobox entry does not open.
func TestComboboxEntryDisabledStaysClosed(t *testing.T) {
	c, host := newTestComboboxEntry(t, ComboboxOption[string]{Value: "a", Label: "a"})
	c.entry.SetEnabled(false)
	c.Open()
	if len(host.popovers) != 0 {
		t.Error("a disabled combobox opened its list")
	}
}

func TestAComboboxEntrysEmptyValueReadsAsTextNotAsAHint(t *testing.T) {
	loadCSS(t, `entry { background-color: #000000; } entry > text { color: #ff0000; } entry > text > placeholder { color: #00ff00; }`)
	c := NewComboboxEntry(entryFace(t), 14)
	c.SetEmpty("Default")
	data, stride, sz := entryShot(t, c.Entry())
	if _, n := inked(data, stride, sz, render.RGB(0xff, 0, 0)); n == 0 {
		t.Error("the empty value is not in the text's color")
	}
	if _, n := inked(data, stride, sz, render.RGB(0, 0xff, 0)); n != 0 {
		t.Errorf("the empty value painted %d px in the placeholder's color", n)
	}
	c.SetEmpty("")
	c.Entry().SetPlaceholder("hint")
	data, stride, sz = entryShot(t, c.Entry())
	if _, n := inked(data, stride, sz, render.RGB(0, 0xff, 0)); n == 0 {
		t.Error("a plain placeholder lost the placeholder's color")
	}
}
