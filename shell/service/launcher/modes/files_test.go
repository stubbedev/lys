package modes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/launchermatch"

	"github.com/stubbedev/lys/shell/service/fileindex"
	"github.com/stubbedev/lys/shell/service/launcher"
)

type fixedIndex struct{ snap *fileindex.Snapshot }

func (f fixedIndex) Snapshot() *fileindex.Snapshot { return f.snap }

// fileFixture is files under home: budget.ods and notes/budget-2026.txt
// indexed, notes/overbudget.md recently used.
func fileFixture(t *testing.T) (home string, idx fixedIndex) {
	t.Helper()
	home = t.TempDir()
	snap := &fileindex.Snapshot{Ready: true, Recent: []string{home + "/notes/overbudget.md"}}
	for _, f := range []struct {
		folder, name string
		dir          bool
	}{{home, "budget.ods", false}, {home, "notes", true}, {home + "/notes", "budget-2026.txt", false}, {home + "/notes", "overbudget.md", false}} {
		snap.Files = append(snap.Files, fileindex.File{Folder: f.folder, Name: f.name, Fold: strings.ToLower(f.name), Dir: f.dir})
		if f.dir {
			continue
		}
		if err := os.MkdirAll(f.folder, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.folder, f.name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home, fixedIndex{snap: snap}
}

func newFilesMode(home string, idx FileIndex, show func(context.Context, string) error) *Files {
	return NewFiles(FilesConfig{Index: idx, Home: home, Negate: '-', ShowItem: show})
}

func infos(items []launcher.Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = *it.Info
	}
	return out
}

func TestFilesFindRecentFilesFirstWithTheirFolderAndThumbnail(t *testing.T) {
	home, idx := fileFixture(t)
	m := newFilesMode(home, idx, nil)
	if st := m.Load(bg); !slices.Equal(infos(st.Items), []string{home + "/notes/overbudget.md"}) {
		t.Errorf("an empty query lists %v, want the recent file", infos(st.Items))
	}
	st, changed := m.Query(bg, "budget")
	if !changed {
		t.Fatal("a query found nothing new")
	}
	want := []string{home + "/notes/overbudget.md", home + "/budget.ods", home + "/notes/budget-2026.txt"}
	if got := infos(st.Items); !slices.Equal(got, want) {
		t.Fatalf("budget lists %v, want %v", got, want)
	}
	row := st.Items[1]
	if row.Display != "budget.ods" || row.Secondary != "~" || row.MatchText != "budget.ods" || row.Flags.Has(launcher.FlagMarkup) {
		t.Errorf("a file row = %+v, want its name over its folder as secondary text", row)
	}
	if icon, ok := row.Icon.(launcher.IconThumbnail); !ok || icon.Path != home+"/budget.ods" || icon.Fallback == "" {
		t.Errorf("a file row's icon = %#v, want its thumbnail over a type icon", row.Icon)
	}
	if st.Items[2].Secondary != "~/notes" || strings.Contains(st.Items[2].Display, "notes") {
		t.Errorf("a file in a folder shows %q over %q", st.Items[2].Display, st.Items[2].Secondary)
	}
	odd := m.item(fileindex.Hit{Path: "/srv/a <b>&amp;.txt", Name: "a <b>&amp;.txt"})
	if odd.Display != "a <b>&amp;.txt" || odd.Secondary != "/srv" || odd.Flags.Has(launcher.FlagMarkup) {
		t.Errorf("a name that reads as markup shows %+v, want it verbatim", odd)
	}
	if _, changed := m.Query(bg, "budget"); changed {
		t.Error("the same files found again changed the list")
	}
	if st, _ := m.Query(bg, "nothing-like-it"); len(st.Items) != 0 {
		t.Errorf("a query matching nothing lists %v", infos(st.Items))
	}
}

func TestFilesKeepTheirListWhenANewerQueryEndsTheSearch(t *testing.T) {
	home, idx := fileFixture(t)
	m := newFilesMode(home, idx, nil)
	m.Query(bg, "budget")
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, changed := m.Query(ctx, "ods"); changed {
		t.Error("an ended search changed the list")
	}
	if len(m.hits) != 3 {
		t.Errorf("an ended search left %d rows, want the 3 before it", len(m.hits))
	}
}

func TestEnterOpensAFileInItsDefaultAppAndAltShowsItInTheFileManager(t *testing.T) {
	home, idx := fileFixture(t)
	var opened, shown []string
	m := NewFiles(FilesConfig{
		Index: idx, Home: home, Negate: '-',
		ShowItem: func(_ context.Context, path string) error {
			shown = append(shown, path)
			return nil
		},
		Open: func(path string) error {
			opened = append(opened, path)
			return nil
		},
	})
	m.Query(bg, "budget.ods")
	wantClosed(t, m.Activate(bg, launcher.RowTarget(0), launcher.ActivateDefault{}, ""))
	if !slices.Equal(opened, []string{home + "/budget.ods"}) {
		t.Errorf("Enter opened %v", opened)
	}
	if len(shown) != 0 {
		t.Errorf("Enter showed %v in the file manager", shown)
	}
	wantClosed(t, m.Activate(bg, launcher.RowTarget(0), launcher.ActivateAlt{}, ""))
	if !slices.Equal(shown, []string{home + "/budget.ods"}) || len(opened) != 1 {
		t.Errorf("the alternate accept showed %v and opened %v", shown, opened)
	}
	if _, ok := m.Activate(bg, launcher.RowTarget(9), launcher.ActivateDefault{}, "").(launcher.ActionNothing); !ok {
		t.Error("a row past the list did something")
	}
	if launcher.AllowsCustom(m) {
		t.Error("typed text that names no file is accepted")
	}
}

// A file whose app cannot be started leaves the list open with a
// message, as a failed show does.
func TestAFileThatCannotBeOpenedLeavesTheListWithAMessage(t *testing.T) {
	home, idx := fileFixture(t)
	m := NewFiles(FilesConfig{Index: idx, Home: home, Negate: '-', Open: func(string) error { return errors.New("no app opens it") }})
	m.Query(bg, "budget.ods")
	r, ok := m.Activate(bg, launcher.RowTarget(0), launcher.ActivateDefault{}, "").(launcher.ActionReload)
	if !ok || r.State.Message == nil || len(r.State.Items) != 1 {
		t.Errorf("a failed open = %#v, want the list again with a message", r)
	}
}

func TestAFileManagerThatFailsLeavesTheListWithAMessage(t *testing.T) {
	home, idx := fileFixture(t)
	m := newFilesMode(home, idx, func(context.Context, string) error { return errors.New("no FileManager1") })
	m.Query(bg, "budget.ods")
	r, ok := m.Activate(bg, launcher.RowTarget(0), launcher.ActivateAlt{}, "").(launcher.ActionReload)
	if !ok || r.State.Message == nil || len(r.State.Items) != 1 {
		t.Errorf("a failed show = %#v, want the list again with a message", r)
	}
}

// combiWithFiles is apps then files, ranked by match quality.
func combiWithFiles(t *testing.T) (*launcher.Session, string) {
	t.Helper()
	home, idx := fileFixture(t)
	c := NewCombi([]launcher.Mode{
		&staticChild{name: "drun", entries: []string{"Budgeting App for Teams", "Notes"}},
		newFilesMode(home, idx, nil),
	}, "{text}")
	opts := launchermatch.DefaultOptions()
	opts.Sort = true
	s, err := launcher.NewSession([]launcher.Mode{c}, opts)
	if err != nil {
		t.Fatal(err)
	}
	s.Load(bg)
	return s, home
}

func matchedTexts(s *launcher.Session) []string {
	var out []string
	for _, i := range s.Matched() {
		out = append(out, s.Items()[i].MatchText)
	}
	return out
}

func TestAppsWinAShortQueryAndFilesCompeteOnALongOne(t *testing.T) {
	s, _ := combiWithFiles(t)
	s.SetQuery(bg, "bud")
	if got := matchedTexts(s); len(got) == 0 || got[0] != "Budgeting App for Teams" || !slices.Contains(got, "overbudget.md") {
		t.Errorf("bud ranks %v, want the app first and the recent file among the rest", got)
	}
	s.SetQuery(bg, "budget.ods")
	if got := matchedTexts(s); !slices.Equal(got, []string{"budget.ods"}) {
		t.Errorf("budget.ods ranks %v", got)
	}
	s.SetQuery(bg, "budget")
	if got := matchedTexts(s); len(got) == 0 || got[0] == "Budgeting App for Teams" {
		t.Errorf("a query of %d runes still puts the app first: %v", TrailRunes+2, got)
	}
}

func TestTheFilesBangKeepsOnlyFiles(t *testing.T) {
	s, _ := combiWithFiles(t)
	s.SetQuery(bg, "!files b")
	if got := matchedTexts(s); !slices.Equal(got, []string{"overbudget.md"}) {
		t.Errorf("!files b = %v, want only the recent file a one-letter query finds", got)
	}
	s.SetQuery(bg, "!files budget")
	if got := matchedTexts(s); len(got) != 3 || slices.Contains(got, "Budgeting App for Teams") {
		t.Errorf("!files budget = %v, want the three files and no app", got)
	}
	s.SetQuery(bg, "!dr budget")
	if got := matchedTexts(s); !slices.Equal(got, []string{"Budgeting App for Teams"}) {
		t.Errorf("!dr budget = %v, want the app alone", got)
	}
}
