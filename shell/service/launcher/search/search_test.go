package search

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/launchermatch"
	"github.com/stubbedev/lys/desktop/pango"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

// fakeProvider offers fixed rows under a section.
type fakeProvider struct {
	spec    Spec
	entries []Entry
	actions []Action
	// gate holds Load until closed, as a slow provider.
	gate chan struct{}
	// published closes once Load has published.
	published chan struct{}
}

func (p *fakeProvider) Spec() Spec { return p.spec }
func (*fakeProvider) Whole()       {}
func (p *fakeProvider) Load(ctx context.Context, publish func([]Entry)) {
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return
		}
	}
	publish(p.entries)
	if p.published != nil {
		close(p.published)
	}
}

func (p *fakeProvider) Actions(Entry) []Action { return p.actions }

// sink records what the panel would be told, each run through the same
// channel the UI loop would drain.
type sink struct {
	mu       sync.Mutex
	frames   []Frame
	finished []Outcome
	notices  []string
}

func (s *sink) Results(f Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames = append(s.frames, f)
}

func (s *sink) Finish(o Outcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = append(s.finished, o)
}

func (s *sink) Notice(m string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notices = append(s.notices, m)
}

func (s *sink) last() Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.frames) == 0 {
		return Frame{}
	}
	return s.frames[len(s.frames)-1]
}

func start(t *testing.T, s *sink, freq *Frecency, providers ...Provider) *Engine {
	t.Helper()
	var open func() (*Frecency, error)
	if freq != nil {
		open = func() (*Frecency, error) { return freq, nil }
	}
	return startLevel(t, s, &Level{Providers: providers}, open)
}

func startLevel(t *testing.T, s *sink, level *Level, open func() (*Frecency, error)) *Engine {
	t.Helper()
	e := Start(func(run func()) { go run() }, level, open, s, func(fn func()) { fn() })
	t.Cleanup(func() { stopped(t, e) })
	return e
}

func stopped(t *testing.T, e *Engine) {
	t.Helper()
	e.Stop()
	waittest.For(t, e.Done(), "the engine to stop")
}

func titles(f Frame) []string {
	var out []string
	for _, r := range f.Rows {
		if r.Header != "" {
			out = append(out, "#"+r.Header)
			continue
		}
		out = append(out, r.Entry.Title)
	}
	return out
}

func until(t *testing.T, s *sink, want ...string) {
	t.Helper()
	waittest.Until(t, func() bool { return slices.Equal(titles(s.last()), want) }, fmt.Sprintf("the rows %q", want))
}

func entry(title string) Entry { return Entry{ID: title, Learn: title, Title: title} }

func tempFrecency(t *testing.T, now func() time.Time) *Frecency {
	t.Helper()
	f, err := OpenFrecency(filepath.Join(t.TempDir(), "state", "f.json"), now)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOneListAcrossProvidersWithSections(t *testing.T) {
	s := &sink{}
	apps := &fakeProvider{spec: Spec{Section: SectionApps, Weight: appWeight}, entries: []Entry{entry("Firefox"), entry("Files")}, published: make(chan struct{})}
	windows := &fakeProvider{spec: Spec{Section: SectionWindows}, entries: []Entry{entry("firefox: news")}, published: make(chan struct{})}
	e := start(t, s, nil, apps, windows)
	<-apps.published
	<-windows.published
	e.SetQuery("firef", 1)
	until(t, s, "#"+SectionApps.Title(), "Firefox", "#"+SectionWindows.Title(), "firefox: news")
	e.SetQuery("zzz", 1)
	until(t, s)
}

func TestSubtitleAndKeywordsMatchBelowTitle(t *testing.T) {
	s := &sink{}
	p := &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{
		{ID: "a", Title: "Browser", Keywords: "web"},
		{ID: "b", Title: "web thing"},
	}}
	e := start(t, s, nil, p)
	e.SetQuery("web", 1)
	until(t, s, "web thing", "Browser")
}

func TestHighlightsMarkTheMatchedCharacters(t *testing.T) {
	s := &sink{}
	e := start(t, s, nil, &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Visual Studio Code")}})
	e.SetQuery("vsc", 1)
	until(t, s, "Visual Studio Code")
	got := s.last().Rows[0].AppendHighlights(nil, launchermatch.NewFuzzyQuery("vsc"))
	if want := []int{0, 7, 14}; !slices.Equal(got, want) {
		t.Errorf("highlights = %v, want the word starts %v", got, want)
	}
}

func TestFrecencyLiftsWhatGetsUsed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	freq := tempFrecency(t, func() time.Time { return now })
	for range 8 {
		if err := freq.Record("Filezilla", ""); err != nil {
			t.Fatal(err)
		}
	}
	s := &sink{}
	e := start(t, s, freq, &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Files"), entry("Filezilla")}})
	e.SetQuery("fil", 1)
	until(t, s, "Filezilla", "Files")
}

func TestUntouchedRanksByMatchNotHistory(t *testing.T) {
	s := &sink{}
	e := start(t, s, nil, &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Filezilla"), entry("Files")}})
	e.SetQuery("fil", 1)
	until(t, s, "Files", "Filezilla")
}

func TestFrecencyLearnsTheQuery(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	freq := tempFrecency(t, func() time.Time { return now })
	if err := freq.Record("Zed", "ze"); err != nil {
		t.Fatal(err)
	}
	if got := boostsFor(freq, "z")["Zed"]; got == 0 {
		t.Error("a query that starts what was typed before should find it")
	}
	if len(boostsFor(freq, "q")) != 0 {
		t.Error("a different query must learn nothing from it")
	}
}

func TestFrecencyDecaysAndPersistsAtomically(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	clock := func() time.Time { return now }
	path := filepath.Join(t.TempDir(), "deep", "f.json")
	f, err := OpenFrecency(path, clock)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Record("a", "query"); err != nil {
		t.Fatal(err)
	}
	fresh := f.Boost("a")
	if fresh == 0 {
		t.Fatal("a use must count")
	}
	listing, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(listing) != 1 {
		t.Errorf("left beside the file: %v", listing)
	}
	now = now.Add(4 * frecencyHalfLife)
	reread, err := OpenFrecency(path, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got := reread.Boost("a"); got == 0 || got >= fresh {
		t.Errorf("boost after four half-lives = %d, fresh %d", got, fresh)
	}
	if err := reread.Forget("a"); err != nil {
		t.Fatal(err)
	}
	if reread.Boost("a") != 0 {
		t.Error("forgotten must be gone")
	}
}

func TestFrecencyCorruptFileIsAnErrorAndAnEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFrecency(path, time.Now)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want one naming the file", err)
	}
	if f == nil || f.Boost("x") != 0 {
		t.Error("the launcher still has to open")
	}
}

func TestFrecencyIsBounded(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	f := tempFrecency(t, func() time.Time { return now })
	for i := range frecencyMaxEntries {
		f.entries[fmt.Sprint("k", i)] = &useRecord{Score: 1, At: now.Unix()}
	}
	if err := f.Record("new", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.Record("new", ""); err != nil {
		t.Fatal(err)
	}
	if len(f.entries) > frecencyMaxEntries || f.Boost("new") == 0 {
		t.Errorf("%d entries, new boost %d", len(f.entries), f.Boost("new"))
	}
}

func TestEmptyQuerySuggestsWhatWasUsed(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	freq := tempFrecency(t, func() time.Time { return now })
	if err := freq.Record("Zed", ""); err != nil {
		t.Fatal(err)
	}
	s := &sink{}
	start(t, s, freq, &fakeProvider{spec: Spec{Section: SectionApps}, entries: []Entry{entry("Abc"), entry("Zed")}})
	until(t, s, "#"+SectionRecent.Title(), "Zed")
}

func TestEmptyQueryAtTheRootShowsNothingWithNoHistory(t *testing.T) {
	s := &sink{}
	p := &fakeProvider{spec: Spec{Section: SectionApps}, entries: []Entry{entry("Abc")}, published: make(chan struct{})}
	e := start(t, s, nil, p)
	<-p.published
	e.SetQuery("", 1)
	waittest.Until(t, func() bool { return s.last().QuerySeq == 1 }, "a frame")
	if rows := s.last().Rows; len(rows) != 0 {
		t.Errorf("rows = %v", rows)
	}
}

func TestLateRowsLeaveWhatIsOnScreenInPlace(t *testing.T) {
	s := &sink{}
	gate := make(chan struct{})
	fast := &fakeProvider{spec: Spec{Section: SectionFiles}, entries: []Entry{entry("alpha one")}}
	slow := &fakeProvider{spec: Spec{Section: SectionApps, Weight: 1000}, entries: []Entry{entry("alpha two")}, gate: gate}
	e := start(t, s, nil, fast, slow)
	e.SetQuery("alpha", 1)
	until(t, s, "#"+SectionFiles.Title(), "alpha one")
	close(gate)
	until(t, s, "#"+SectionFiles.Title(), "alpha one", "#"+SectionApps.Title(), "alpha two")
	if !s.last().Late {
		t.Error("the frame must say its rows came late, so the selection stays")
	}
}

func TestSectionsDoNotSwapOnASmallLead(t *testing.T) {
	want := []Section{SectionApps, SectionFiles}
	sections := []sectionRows{
		{section: SectionFiles, rows: []ranked{{score: 105}}},
		{section: SectionApps, rows: []ranked{{score: 100}}},
	}
	if got := orderSections(sections, slices.Clone(want), false); !slices.Equal(got, want) {
		t.Errorf("a small lead swapped them: %v", got)
	}
	sections = []sectionRows{
		{section: SectionFiles, rows: []ranked{{score: 100 + sectionStickiness + 1}}},
		{section: SectionApps, rows: []ranked{{score: 100}}},
	}
	if got := orderSections(sections, slices.Clone(want), false); got[0] != SectionFiles {
		t.Errorf("a clear lead must win: %v", got)
	}
}

func TestAnswerLeadsAndWebClosesTheList(t *testing.T) {
	s := &sink{}
	web := NewWeb(Env{}, "E", "https://e/?q={query}")
	e := start(t, s, nil, web, testCalculator(t, nil), &fakeProvider{spec: Spec{Section: SectionApps}, entries: []Entry{entry("2+2 app")}})
	e.SetQuery("2+2", 1)
	waittest.Until(t, func() bool { return len(s.last().Rows) >= 5 }, "rows")
	got := titles(s.last())
	if got[1] != "4" {
		t.Errorf("first row = %v", got)
	}
	if !strings.Contains(got[len(got)-1], "E") {
		t.Errorf("last row = %v", got)
	}
}

func TestWebTemplateWithoutQueryOffersNothing(t *testing.T) {
	if got := NewWeb(Env{}, "E", "https://e/").Answer("x"); got != nil {
		t.Errorf("answer = %v", got)
	}
	got := NewWeb(Env{}, "E", "https://e/?q={query}").Answer("a b&c")
	if len(got) != 1 || got[0].Data != "https://e/?q=a+b%26c" {
		t.Errorf("answer = %+v, want it escaped", got)
	}
}

func TestActionsLevelAndBackRestoreTheQuery(t *testing.T) {
	s := &sink{}
	ran := make(chan string, 1)
	p := &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Thing")}, actions: []Action{
		{Title: "Open", Run: func(context.Context) Outcome { ran <- "open"; return Close{} }},
		{Title: "Copy", Run: func(context.Context) Outcome { ran <- "copy"; return Copy{Text: "x"} }},
	}}
	e := start(t, s, nil, p)
	e.SetQuery("thi", 1)
	until(t, s, "Thing")
	e.ShowActions("Thing")
	until(t, s, "Open", "Copy")
	if f := s.last(); f.Depth != 1 || f.Title != "Thing" || !f.SetField || f.Field != "" {
		t.Errorf("actions frame = %+v", f)
	}
	e.SetQuery("cop", 1)
	until(t, s, "Copy")
	e.Back()
	until(t, s, "Thing")
	if f := s.last(); f.Depth != 0 || !f.SetField || f.Field != "thi" {
		t.Errorf("back frame = %+v", f)
	}
	e.ShowActions("Thing")
	until(t, s, "Open", "Copy")
	e.Activate("1", 0)
	if got := waittest.For(t, ran, "the action to run"); got != "copy" {
		t.Errorf("ran %s", got)
	}
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.finished) == 1
	}, "finish")

	if c, ok := s.finished[0].(Copy); !ok || c.Text != "x" {
		t.Errorf("finished %v", s.finished)
	}
}

func TestBackAtTheRootCloses(t *testing.T) {
	s := &sink{}
	e := start(t, s, nil)
	e.Back()
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.finished) == 1
	}, "finish")
}

func TestActivatingRecordsTheUse(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	freq := tempFrecency(t, func() time.Time { return now })
	s := &sink{}
	done := make(chan struct{})
	p := &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Thing")}, actions: []Action{
		{Title: "Open", Run: func(context.Context) Outcome { close(done); return Close{} }},
	}}
	e := start(t, s, freq, p)
	e.SetQuery("thi", 1)
	until(t, s, "Thing")
	e.Activate("Thing", 0)
	<-done
	if freq.Boost("Thing") == 0 {
		t.Error("the use did not count")
	}
	data, err := os.ReadFile(freq.path)
	if err != nil || !strings.Contains(string(data), "Thing") {
		t.Errorf("file: %s, %v", data, err)
	}
}

func TestAFailedActionSaysItsSummaryAndLogsItsError(t *testing.T) {
	logged := logtest.Capture(t)
	s := &sink{}
	p := &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("Thing")}, actions: []Action{
		{Title: "Open", Run: func(context.Context) Outcome {
			return Failed{Summary: "Thing failed", Err: errors.New("exec: thing: not found")}
		}},
	}}
	e := start(t, s, nil, p)
	e.SetQuery("thi", 1)
	until(t, s, "Thing")
	e.Activate("Thing", 0)
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.notices) == 1
	}, "the failure said")
	s.mu.Lock()
	notices, finished := slices.Clone(s.notices), len(s.finished)
	s.mu.Unlock()
	if notices[0] != "Thing failed" || finished != 0 {
		t.Errorf("notices %q with %d finishes, want the summary alone and the panel kept", notices, finished)
	}
	if got := logged.String(); !strings.Contains(got, "launcher: Thing failed: exec: thing: not found") {
		t.Errorf("log %q, want the summary with its error", got)
	}
}

func TestOvertakenQueriesAreSkippedButAcceptsSeeTheirText(t *testing.T) {
	q := func(s string) engineCmd { return engineCmd{kind: cmdQuery, text: s} }
	a := q("a")
	if !overtaken(&a, []engineCmd{q("ab")}) {
		t.Error("an adjacent newer query overtakes")
	}
	if overtaken(&a, []engineCmd{{kind: cmdActivate, id: "x"}, q("ab")}) {
		t.Error("an accept between them must see the older text")
	}
	if overtaken(&a, nil) {
		t.Error("the last query stands")
	}
}

func TestOnlyAPublishOfTheSameOrALaterLoadOvertakesOne(t *testing.T) {
	c := &corpus{}
	publish := func(load uint64) engineCmd { return engineCmd{kind: cmdPublish, corpus: c, load: load} }
	older := publish(1)
	if !overtaken(&older, []engineCmd{publish(2)}) {
		t.Error("a publish of the reload must overtake one of the load before")
	}
	same := publish(2)
	if !overtaken(&same, []engineCmd{publish(2)}) {
		t.Error("a later publish of the same load must overtake")
	}
	newer := publish(2)
	if overtaken(&newer, []engineCmd{publish(1)}) {
		t.Error("a stale publish behind the reload's must not overtake it")
	}
	other := publish(1)
	if overtaken(&other, []engineCmd{{kind: cmdPublish, corpus: &corpus{}, load: 1}}) {
		t.Error("another corpus's publish must not overtake")
	}
}

type scriptedLoad struct {
	entries []Entry
	gate    chan struct{}
}

type scriptedProvider struct {
	spec      Spec
	mu        sync.Mutex
	loads     []scriptedLoad
	published chan int
}

func (p *scriptedProvider) Spec() Spec           { return p.spec }
func (*scriptedProvider) Actions(Entry) []Action { return nil }
func (p *scriptedProvider) Load(_ context.Context, publish func([]Entry)) {
	p.mu.Lock()
	if len(p.loads) == 0 {
		p.mu.Unlock()
		panic("scriptedProvider: a load nothing scripted")
	}
	l := p.loads[0]
	p.loads = p.loads[1:]
	p.mu.Unlock()
	if l.gate != nil {
		<-l.gate
	}
	publish(l.entries)
	p.published <- len(l.entries)
}

func reloading(title string) *fakeProvider {
	p := &fakeProvider{spec: Spec{Section: SectionClipboard}, entries: []Entry{entry(title)}}
	p.actions = []Action{{Title: "forget", Run: func(context.Context) Outcome { return Reload{} }}}
	return p
}

func untilQuery(t *testing.T, s *sink, query string) Frame {
	t.Helper()
	waittest.Until(t, func() bool { return s.last().Query == query }, "the frame for "+query)
	return s.last()
}

func TestAReloadKeepsTheRowsOfProvidersStillLoading(t *testing.T) {
	s := &sink{}
	held := make(chan struct{})
	links := &scriptedProvider{
		spec:      Spec{Section: SectionLinks},
		loads:     []scriptedLoad{{entries: []Entry{entry("alpha docs")}}, {entries: []Entry{entry("alpha docs")}, gate: held}},
		published: make(chan int, 2),
	}
	e := start(t, s, nil, links, reloading("zulu"))
	waittest.For(t, links.published, "the first load of the links")
	e.SetQuery("zulu", 1)
	until(t, s, "#"+SectionClipboard.Title(), "zulu")
	e.Activate("zulu", 0)
	e.SetQuery("alpha", 2)
	if got, want := titles(untilQuery(t, s, "alpha")), []string{"#" + SectionLinks.Title(), "alpha docs"}; !slices.Equal(got, want) {
		t.Errorf("rows while the links load again = %q, want the links they had, %q", got, want)
	}
	close(held)
	waittest.For(t, links.published, "the reload of the links")
}

func TestALoadAReloadOvertookPublishesNothing(t *testing.T) {
	s := &sink{}
	stale := make(chan struct{})
	links := &scriptedProvider{
		spec:      Spec{Section: SectionLinks},
		loads:     []scriptedLoad{{entries: []Entry{entry("alpha old")}, gate: stale}, {entries: []Entry{entry("alpha new")}}},
		published: make(chan int, 2),
	}
	e := start(t, s, nil, links, reloading("zulu"))
	e.SetQuery("zulu", 1)
	until(t, s, "#"+SectionClipboard.Title(), "zulu")
	e.Activate("zulu", 0)
	waittest.For(t, links.published, "the reload of the links")
	close(stale)
	waittest.For(t, links.published, "the load the reload overtook")
	e.SetQuery("alpha", 2)
	if got, want := titles(untilQuery(t, s, "alpha")), []string{"#" + SectionLinks.Title(), "alpha new"}; !slices.Equal(got, want) {
		t.Errorf("rows after a stale load published = %q, want the reload's, %q", got, want)
	}
}

func TestStopCancelsProvidersAndLeavesNothingRunning(t *testing.T) {
	s := &sink{}
	gate := make(chan struct{})
	defer close(gate)
	e := Start(func(run func()) { go run() }, &Level{Providers: []Provider{&fakeProvider{spec: Spec{Section: SectionApps}, gate: gate}}}, nil, s, func(fn func()) { fn() })
	e.Stop()
	waittest.For(t, e.Done(), "a provider to let the engine stop")
}

func TestConvert(t *testing.T) {
	en := loaderIn(t, "en-US")
	for in, want := range map[string]string{
		"5 km to mi":      "3.1068559612 mi",
		"100 f to c":      "37.7777777778°C",
		"0 c in f":        "32°F",
		"1 GB to MB":      "1,000 MB",
		"1 GiB to MiB":    "1,024 MiB",
		"8 Mbit to MB":    "1 MB",
		"2*(3+4)km to m":  "14,000 m",
		"90 min to hours": "1.5 h",
		"1 kg in lb":      "2.2046226218 lb",
		"1 mph to km/h":   "1.609344 km/h",
		"180 deg to rad":  "3.1415926536 rad",
	} {
		got, ok := modes.Convert(in)
		if !ok || got.Show(en) != want {
			t.Errorf("%q = %q (%v), want %q", in, got.Show(en), ok, want)
		}
	}
	for _, in := range []string{"5 km to kg", "5 km to", "km to mi", "go to mi", "5 km", "hello in there"} {
		if got, ok := modes.Convert(in); ok {
			t.Errorf("%q converted to %v", in, got)
		}
	}
}

func TestCalculatorAnswersOnlyWhatLooksLikeASum(t *testing.T) {
	c := testCalculator(t, nil)
	for _, q := range []string{"2+2", "sqrt(16)", "5 km to mi", "10 % 3"} {
		if len(c.Answer(q)) != 1 {
			t.Errorf("%q: no answer", q)
		}
	}
	for _, q := range []string{"12", "e", "pi", "firefox", ""} {
		if got := c.Answer(q); got != nil {
			t.Errorf("%q answered %v", q, got)
		}
	}
}

func TestParseScript(t *testing.T) {
	src := `#!/bin/sh
# @lys.title Say Hello
# @lys.subtitle Greets
# @lys.keywords greet hi
# @lys.output copy
# @lys.argument name
# @lys.argument greeting optional
echo hi
# @lys.title Not read past the header
`
	s, ok, err := ParseScript("/x", strings.NewReader(src))
	if err != nil || !ok {
		t.Fatalf("ok %v err %v", ok, err)
	}
	if s.Title != "Say Hello" || s.Output != ScriptCopy || s.Keywords != "greet hi" || len(s.Args) != 2 || s.Args[0].Optional || !s.Args[1].Optional {
		t.Errorf("parsed %+v", s)
	}
	if _, ok, err := ParseScript("/x", strings.NewReader("#!/bin/sh\necho hi\n")); ok || err != nil {
		t.Errorf("no title: ok %v err %v", ok, err)
	}
	for _, bad := range []string{"# @lys.output loud\n", "# @lys.nope x\n", "# @lys.argument a b c\n", "# @lys.argument\n"} {
		_, _, err := ParseScript("/x", strings.NewReader(bad))
		if err == nil || !strings.Contains(err.Error(), "/x:1") {
			t.Errorf("%q: err = %v, want one naming the line", bad, err)
		}
	}
}

func TestScriptArgumentsStayData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c")
	body := "#!/bin/sh\n# @lys.title C\n# @lys.output copy\n# @lys.argument a\nprintf '%s' \"$1\"\n"
	fdtest.WriteExecutable(t, path, []byte(body))
	s := &sink{}
	e := start(t, s, nil, NewScripts(Env{}, dir))
	e.SetQuery("c", 1)
	waittest.Until(t, func() bool { return len(titles(s.last())) > 0 }, "row")
	id := ""
	for _, r := range s.last().Rows {
		if r.Header == "" {
			id = r.Entry.ID
		}
	}
	e.Activate(id, 0)
	waittest.Until(t, func() bool { return s.last().CanSubmit }, "the argument level")
	e.Submit("")
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.notices) == 1
	}, "the refusal")

	e.Submit("; rm -rf $HOME")
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.finished) == 1
	}, "finish")

	if c, ok := s.finished[0].(Copy); !ok || c.Text != "; rm -rf $HOME" {
		t.Errorf("finished %+v", s.finished[0])
	}
}

func TestTypedPathListsItsDirectory(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"Documents", "Downloads", "Pictures", ".hidden"} {
		if err := os.Mkdir(filepath.Join(home, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	f := NewFiles(Env{}, FilesConfig{Home: home})
	got := f.Answer("~/do")
	if len(got) != 2 || got[0].Completion != "~/Documents/" {
		t.Fatalf("answer = %+v", got)
	}
	if rows := f.Answer("~/"); len(rows) != 3 {
		t.Errorf("hidden shows only when asked for: %+v", rows)
	}
	if f.Answer("documents") != nil {
		t.Error("only a path is a path")
	}
	if f.Answer("/does/not/exist/x") != nil {
		t.Error("a missing directory answers nothing")
	}
}

// fakeWindows records what the list asks of the compositor.
type fakeWindows struct {
	list    []modes.WindowInfo
	focused []string
	closed  []string
}

func (w *fakeWindows) List(context.Context) ([]modes.WindowInfo, error) { return w.list, nil }
func (w *fakeWindows) Focus(_ context.Context, id string) error {
	w.focused = append(w.focused, id)
	return nil
}

func (w *fakeWindows) Close(_ context.Context, id string) error {
	w.closed = append(w.closed, id)
	return nil
}

func TestWindowsLeaveOutTheFocusedAndJump(t *testing.T) {
	back := &fakeWindows{list: []modes.WindowInfo{
		{ID: "1", Title: "here", Class: "foot", Focused: true},
		{ID: "2", Title: "there", Class: "Firefox", Workspace: "3"},
	}}
	w := NewWindows(back)
	var got []Entry
	w.Load(context.Background(), func(e []Entry) { got = e })
	if len(got) != 1 || got[0].Title != "there" || got[0].Learn != "window:firefox" {
		t.Fatalf("entries %+v", got)
	}
	actions := w.Actions(got[0])
	if _, ok := actions[0].Run(context.Background()).(Close); !ok || !slices.Equal(back.focused, []string{"2"}) {
		t.Errorf("focused %v", back.focused)
	}
	if _, ok := actions[1].Run(context.Background()).(Close); !ok || !slices.Equal(back.closed, []string{"2"}) {
		t.Errorf("closed %v", back.closed)
	}
	var none []Entry
	NewWindows(nil).Load(context.Background(), func(e []Entry) { none = e })
	if none != nil {
		t.Errorf("no compositor offered %v", none)
	}
}

func TestSystemConfirmsWhatEndsTheSession(t *testing.T) {
	var ran []string
	env := Env{SpawnShell: func(l string) { ran = append(ran, l) }}
	p := config.DefaultsPower()
	p.ShowLock, p.ShowReboot, p.LockCommand, p.RebootCommand = true, true, config.ShellPowerCommand("lockit"), config.ShellPowerCommand("rebootit")
	sys := NewSystem(env, p)
	var entries []Entry
	sys.Load(context.Background(), func(e []Entry) { entries = e })
	find := func(id string) Entry {
		for _, e := range entries {
			if e.ID == id {
				return e
			}
		}
		t.Fatalf("no %s in %+v", id, entries)
		return Entry{}
	}
	if _, ok := sys.Actions(find("system:lock"))[0].Run(context.Background()).(Close); !ok || !slices.Equal(ran, []string{"lockit"}) {
		t.Errorf("lock ran %v", ran)
	}
	ran = nil
	push, ok := sys.Actions(find("system:reboot"))[0].Run(context.Background()).(Push)
	if !ok || len(ran) != 0 {
		t.Fatalf("reboot must ask first: ran %v", ran)
	}
	var confirm []Entry
	prov := push.Level.Providers[0]
	prov.Load(context.Background(), func(e []Entry) { confirm = e })
	prov.Actions(confirm[0])[0].Run(context.Background())
	if !slices.Equal(ran, []string{"rebootit"}) {
		t.Errorf("confirmed ran %v", ran)
	}
}

// The default session commands are built-ins: they run in process
// through RunPower and start nothing; a refusal stays in the panel,
// said as the action's failure.
func TestSystemRunsTheBuiltinsInProcess(t *testing.T) {
	var spawned []string
	var asked []config.PowerBuiltin
	var refusal error
	env := Env{
		SpawnShell: func(l string) { spawned = append(spawned, l) },
		RunPower: func(_ context.Context, b config.PowerBuiltin) error {
			asked = append(asked, b)
			return refusal
		},
	}
	sys := NewSystem(env, config.DefaultsPower())
	var entries []Entry
	sys.Load(context.Background(), func(e []Entry) { entries = e })
	lock := slices.IndexFunc(entries, func(e Entry) bool { return e.ID == "system:lock" })
	if lock < 0 {
		t.Fatalf("no lock in %+v", entries)
	}
	if _, ok := sys.Actions(entries[lock])[0].Run(context.Background()).(Close); !ok || !slices.Equal(asked, []config.PowerBuiltin{config.PowerBuiltinLock}) {
		t.Errorf("lock asked %v", asked)
	}
	refusal = errors.New("access denied")
	out, ok := sys.Actions(entries[lock])[0].Run(context.Background()).(Failed)
	if !ok || !errors.Is(out.Err, refusal) || out.Summary != i18n.T("launcher-failed", i18n.Str("what", entries[lock].Title)) {
		t.Errorf("a refused lock answered %#v", out)
	}
	if len(spawned) != 0 {
		t.Errorf("the built-ins started %v", spawned)
	}
	sys = NewSystem(Env{SpawnShell: env.SpawnShell}, config.DefaultsPower())
	if out, ok := sys.Actions(entries[lock])[0].Run(context.Background()).(Failed); !ok || !errors.Is(out.Err, errNoPowerRunner) {
		t.Errorf("with no runner the lock answered %#v", out)
	}
}

// The confirm level's message holds the action's title as text,
// whatever the title has of markup.
func TestTheConfirmMessageHoldsTheTitleAsText(t *testing.T) {
	title := `Reboot <b>now</b> & lose "work"`
	push, ok := confirmed(Action{Title: title}).Run(context.Background()).(Push)
	if !ok {
		t.Fatal("a confirmed action must ask first")
	}
	if text, ok := pango.PlainText(push.Level.Message); !ok || !strings.Contains(text, title) {
		t.Errorf("the message %q reads %q (markup parsed: %v), want the title as text: %q", push.Level.Message, text, ok, title)
	}
}

// scriptedMode is a rofi mode whose reload changes what it shows.
type scriptedMode struct{ unlocked bool }

func (*scriptedMode) Name() string { return "m" }
func (m *scriptedMode) Load(context.Context) launcher.ModeState {
	if !m.unlocked {
		msg := "locked"
		return launcher.ModeState{Password: true, Message: &msg}
	}
	return launcher.ModeState{Items: []launcher.Item{{Display: "<b>item</b>", MatchText: "item"}}, MarkupRows: true, NoCustom: true}
}

func (m *scriptedMode) Activate(ctx context.Context, target launcher.Target, kind launcher.ActivateKind, _ string) launcher.Action {
	if c, ok := kind.(launcher.ActivateCustom); ok && c.Text == "pw" {
		m.unlocked = true
		return launcher.ActionReload{State: m.Load(ctx)}
	}
	if _, ok := target.Row(); ok {
		return launcher.ActionCopy{Text: "secret"}
	}
	return launcher.ActionNothing{}
}

func TestModeLevelHostsAModeWithItsPrompts(t *testing.T) {
	s := &sink{}
	e := start(t, s, nil, &fakeProvider{spec: Spec{Section: SectionNone}, entries: []Entry{entry("vault")}, actions: []Action{
		{Title: "open", Run: func(context.Context) Outcome { return Push{Level: ModeLevel(&scriptedMode{}, "Vault")} }},
	}})
	e.SetQuery("vault", 1)
	until(t, s, "vault")
	e.Activate("vault", 0)
	waittest.Until(t, func() bool { return s.last().Mask }, "the masked prompt")
	if f := s.last(); f.Message != "locked" || !f.CanSubmit || f.Title != "Vault" {
		t.Errorf("prompt frame %+v", f)
	}
	e.Submit("pw")
	waittest.Until(t, func() bool { return !s.last().Mask && len(s.last().Rows) == 1 }, "the items")
	f := s.last()
	if f.Rows[0].Entry.Title != "item" || f.CanSubmit {
		t.Errorf("items frame %+v", f)
	}
	e.Activate(f.Rows[0].Entry.ID, 0)
	waittest.Until(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.finished) == 1
	}, "finish")

	if c, ok := s.finished[0].(Copy); !ok || c.Text != "secret" {
		t.Errorf("finished %+v", s.finished)
	}
}

func TestNewRootHonoursTheProviderList(t *testing.T) {
	cfg := config.DefaultsLauncher().Search
	root := NewRoot(cfg, config.DefaultsPower(), Deps{Env: Env{}}, t.TempDir())
	if len(root.Providers) < 7 {
		t.Errorf("defaults gave %d providers", len(root.Providers))
	}
	cfg.Providers = []config.LauncherSearchProvider{config.SearchWeb}
	root = NewRoot(cfg, config.DefaultsPower(), Deps{Env: Env{}}, t.TempDir())
	if len(root.Providers) != 1 {
		t.Errorf("providers = %d", len(root.Providers))
	}
}

func boostsFor(f *Frecency, q string) map[string]int {
	m := map[string]int{}
	f.QueryBoostsInto(m, q)
	return m
}

func TestASectionKeepsItsCapHoweverManyProvidersFillIt(t *testing.T) {
	rows := func(prefix string) *fakeProvider {
		p := &fakeProvider{spec: Spec{Section: SectionSettings}}
		for i := range 9 {
			p.entries = append(p.entries, entry(fmt.Sprintf("needle %s%d", prefix, i)))
		}
		return p
	}
	short := &fakeProvider{spec: Spec{Section: SectionSettings}, entries: []Entry{entry("needle")}}
	s := &sink{}
	e := start(t, s, nil, rows("a"), rows("b"), short)
	e.SetQuery("needle", 1)
	until(t, s, "#Settings", "needle", "needle a0", "needle a1", "needle a2")
	if SectionSettings.rowCap() != 4 {
		t.Errorf("the expected rows assume a cap of 4, not %d", SectionSettings.rowCap())
	}
}
