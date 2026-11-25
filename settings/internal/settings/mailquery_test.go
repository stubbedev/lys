package settings

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAMailQueryReadsAsConditionsAndWritesBack(t *testing.T) {
	terms, ok := parseQuery(`tag:unread and not tag:spam and folder:"work/My Box"`)
	want := []queryTerm{{field: "tag", value: "unread"}, {field: "tag", value: "spam", negate: true}, {field: "folder", value: "work/My Box"}}
	if !ok || !slices.Equal(terms, want) {
		t.Fatalf("%+v, %v", terms, ok)
	}
	if got := joinQuery(terms); got != `tag:unread and not tag:spam and folder:"work/My Box"` {
		t.Errorf("wrote back %q", got)
	}
	if terms, ok := parseQuery(""); !ok || len(terms) != 0 {
		t.Errorf("an empty query reads as %+v, %v", terms, ok)
	}
	for _, q := range []string{"tag:a or tag:b", "from:alice", "(tag:a and tag:b)", "tag:a and"} {
		if _, ok := parseQuery(q); ok {
			t.Errorf("%q read as conditions", q)
		}
	}
}

func TestTheMailFoldersAreTheMaildirs(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"INBOX/cur", "work/INBOX/cur", "work/notes", ".notmuch/xapian/cur"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := values(maildirFolders(root)); !slices.Equal(got, []string{"INBOX", "work/INBOX"}) {
		t.Fatalf("%q", got)
	}
	if got := maildirFolders(filepath.Join(root, "missing")); len(got) != 0 {
		t.Fatalf("a missing database lists %v", got)
	}
}

func TestTheMailQueryIsBuiltFromConditions(t *testing.T) {
	const path = "modules.mail.query"
	k := testKit(t, "modules:\n  mail:\n    query: \"tag:unread\"\n")
	e := fieldControl(k, k.store, field(path, mailQueryRow)).(*mailQueryEditor)
	if e.chips.Len() != 1 || e.raw.Visible() || e.notice.Visible() {
		t.Fatalf("%d chips, raw %v, notice %v", e.chips.Len(), e.raw.Visible(), e.notice.Visible())
	}
	opts := values(e.options())
	if slices.Contains(opts, "tag:unread") || !slices.Contains(opts, "not tag:spam") || !slices.Contains(opts, "folder:work/INBOX") {
		t.Fatalf("offered %q", opts)
	}
	k.pickers = &fakePickers{}
	e.add.ClickAt(widget.Point{})
	pickName(t, e.picker, "not tag:spam")
	if v := k.store.value(path); v != "tag:unread and not tag:spam" {
		t.Fatalf("adding a condition stored %v", v)
	}
	e.terms = e.terms[1:]
	e.write()
	if v := k.store.value(path); v != "not tag:spam" {
		t.Errorf("removing a condition stored %v", v)
	}
	if err := k.store.set(path, "from:alice or tag:x"); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	if !e.notice.Visible() || !e.raw.Visible() || e.add.Visible() {
		t.Errorf("a query beyond conditions shows notice %v, raw %v, add %v", e.notice.Visible(), e.raw.Visible(), e.add.Visible())
	}
}

func TestTheMailQuerySaysHowManyMessagesItMatches(t *testing.T) {
	const path = "modules.mail.query"
	k := testKit(t, "modules:\n  mail:\n    query: \"tag:unread\"\n")
	counts := map[string]uint32{"tag:unread": 37, "tag:unread and not tag:spam": 1, "tag:none": 0}
	var asked []string
	k.mailCount = func(q string) (uint32, error) {
		asked = append(asked, q)
		n, ok := counts[q]
		if !ok {
			return 0, errors.New("unbalanced parenthesis")
		}
		return n, nil
	}
	e := fieldControl(k, k.store, field(path, mailQueryRow)).(*mailQueryEditor)
	if !e.count.Visible() || unisolated(e.count.Text()) != "37 messages match" {
		t.Fatalf("shows %q, visible %v", e.count.Text(), e.count.Visible())
	}
	e.refresh()
	if len(asked) != 1 {
		t.Errorf("an unchanged query was counted %d times", len(asked))
	}
	for q, want := range map[string]string{
		"tag:unread and not tag:spam": "1 message matches",
		"tag:none":                    "No message matches",
	} {
		if err := k.store.set(path, q); err != nil {
			t.Fatal(err)
		}
		e.refresh()
		if unisolated(e.count.Text()) != want || e.count.kind == stateError {
			t.Errorf("%q shows %v %q, want %q", q, e.count.kind, e.count.Text(), want)
		}
	}
	if err := k.store.set(path, "tag:("); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	if !failedWith(e.count, locale.T().Get("settings-mail-query-count-failed"), "unbalanced parenthesis") {
		t.Errorf("a query notmuch refuses shows %q over %q", e.count.Text(), e.count.detail.Text())
	}
	if err := k.store.set(path, ""); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	if e.count.Visible() {
		t.Error("an empty query shows a count")
	}
}

func TestALateMailCountOfAnOldQueryIsDropped(t *testing.T) {
	const path = "modules.mail.query"
	k := testKit(t, "modules:\n  mail:\n    query: \"tag:old\"\n")
	var pending []func()
	k.background = func(fn func()) { pending = append(pending, fn) }
	k.mailCount = func(q string) (uint32, error) {
		if q == "tag:old" {
			return 5, nil
		}
		return 2, nil
	}
	e := fieldControl(k, k.store, field(path, mailQueryRow)).(*mailQueryEditor)
	if err := k.store.set(path, "tag:new"); err != nil {
		t.Fatal(err)
	}
	e.refresh()
	for _, fn := range slices.Backward(pending) {
		fn()
	}
	if unisolated(e.count.Text()) != "2 messages match" {
		t.Fatalf("shows %q after the old query's count came late", e.count.Text())
	}
}

// Mail's Query "+" stood at its line's top while a format's add centres
// on its chips. Both centre on their chips' first line.
func TestAMailQuerysAddCentresOnItsChipsFirstLine(t *testing.T) {
	k := testKit(t, "modules:\n  mail:\n    query: \"tag:unread and not tag:spam\"\n")
	e := newMailQueryEditor(k, pathSlot(k.store, "modules.mail.query"), false)
	root := widget.NewBox(widget.Column, 0, 0)
	root.Append(e, false)
	styledLayOut(k, root, 700)
	if e.chips.Len() == 0 {
		t.Fatal("the query shows no chips")
	}
	if chip, add := centreY(e.chips.ChipAt(0).Bounds()), centreY(e.add.Bounds()); chip != add {
		t.Errorf("the first chip centres at y %d, the add at %d", chip, add)
	}
}
