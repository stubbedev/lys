package settings

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
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
