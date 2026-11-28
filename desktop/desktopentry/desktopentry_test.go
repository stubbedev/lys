package desktopentry

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/i18n"
	"github.com/stubbedev/lys/lib/waittest"
)

func write(t *testing.T, dir, rel, body string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setLocale(t *testing.T, env map[string]string) {
	t.Helper()
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, env[k])
	}
}

func localizedName(t *testing.T, env map[string]string, body string) string {
	t.Helper()
	setLocale(t, env)
	kf, err := ParseKeyFile([]byte("[Desktop Entry]\nName=Files\n" + body))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := kf.LocaleString(DesktopGroup, "Name")
	return v
}

func TestKeyFileStringsListsAndLocales(t *testing.T) {
	if v := localizedName(t, map[string]string{"LANG": "de_DE.UTF-8"}, "Name[de]=Dateien\n"); v != "Dateien" {
		t.Errorf("localized name = %q", v)
	}
	if v := localizedName(t, map[string]string{"LANG": "fr_FR"}, "Name[de]=Dateien\n"); v != "Files" {
		t.Errorf("fallback name = %q", v)
	}
	kf, err := ParseKeyFile([]byte("# c\n[Desktop Entry]\nComment=a\\sb\\nc\nKeywords=one;two\\;half;three;\nNoDisplay=true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := kf.String(DesktopGroup, "Comment"); v != "a b\nc" {
		t.Errorf("escapes = %q", v)
	}
	if got := kf.List(DesktopGroup, "Keywords"); !reflect.DeepEqual(got, []string{"one", "two;half", "three"}) {
		t.Errorf("list = %q", got)
	}
	if !kf.Bool(DesktopGroup, "NoDisplay") || kf.Bool(DesktopGroup, "Missing") {
		t.Error("booleans")
	}
}

func TestMalformedKeyFilesAreRefused(t *testing.T) {
	for _, body := range []string{"Name=x\n", "[Desktop Entry]\nnot a pair\n", "[Desktop Entry\n"} {
		if _, err := ParseKeyFile([]byte(body)); err == nil {
			t.Errorf("%q must not parse", body)
		}
	}
}

func TestLanguagesWiden(t *testing.T) {
	setLocale(t, map[string]string{"LANG": "sr_RS.UTF-8@latin"})
	if got := i18n.Locales(); !reflect.DeepEqual(got, []string{"sr_RS@latin", "sr_RS", "sr@latin", "sr"}) {
		t.Errorf("got %q", got)
	}
	for _, l := range []string{"", "C", "C.UTF-8", "POSIX"} {
		setLocale(t, map[string]string{"LANG": l})
		if got := i18n.Locales(); got != nil {
			t.Errorf("%q = %q, want none", l, got)
		}
	}
}

func TestEntryNamesFollowLANGUAGEBeforeLANG(t *testing.T) {
	const names = "Name[de]=Dateien\nName[en_US]=Files (US)\nName[nl]=Bestanden\n"
	if v := localizedName(t, map[string]string{"LANGUAGE": "de:en", "LANG": "en_US.UTF-8"}, names); v != "Dateien" {
		t.Errorf("LANGUAGE=de:en, LANG=en_US: name = %q, want the German one", v)
	}
	if v := localizedName(t, map[string]string{"LANGUAGE": "ja:nl_BE", "LANG": "en_US.UTF-8"}, names); v != "Bestanden" {
		t.Errorf("LANGUAGE=ja:nl_BE, LANG=en_US: name = %q, want the Dutch one", v)
	}
	if v := localizedName(t, map[string]string{"LANGUAGE": "ja", "LANG": "en_US.UTF-8"}, names); v != "Files (US)" {
		t.Errorf("LANGUAGE=ja, LANG=en_US: name = %q, want LANG's after the untranslated ja", v)
	}
}

func TestEntryNamesWithoutLANGUAGEFollowTheFirstSetCategory(t *testing.T) {
	const names = "Name[de]=Dateien\nName[en_US]=Files (US)\nName[nl]=Bestanden\n"
	for _, c := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LC_ALL": "nl_NL.UTF-8", "LC_MESSAGES": "de_DE.UTF-8", "LANG": "en_US.UTF-8"}, "Bestanden"},
		{map[string]string{"LC_MESSAGES": "de_DE.UTF-8", "LANG": "en_US.UTF-8"}, "Dateien"},
		{map[string]string{"LANG": "en_US.UTF-8"}, "Files (US)"},
		{map[string]string{"LC_ALL": "C", "LANG": "de_DE.UTF-8"}, "Files"},
		{map[string]string{}, "Files"},
	} {
		if v := localizedName(t, c.env, names); v != c.want {
			t.Errorf("%v: name = %q, want %q", c.env, v, c.want)
		}
	}
}

func appsFixture(t *testing.T) (string, string) {
	t.Helper()
	bin := t.TempDir()
	tool := filepath.Join(bin, "mytool")
	fdtest.WriteExecutable(t, tool, []byte("#!/bin/sh\n"))
	t.Setenv("PATH", bin)
	dir := t.TempDir()
	write(t, dir, "mytool.desktop", "[Desktop Entry]\nType=Application\nName=My Tool\nExec=mytool %U\nIcon=mytool\nActions=new;ghost;\n\n[Desktop Action new]\nName=New Window\nExec=mytool --new\n")
	write(t, dir, "kde/sub.desktop", "[Desktop Entry]\nType=Application\nName=Sub\nExec=mytool\n")
	write(t, dir, "missing.desktop", "[Desktop Entry]\nType=Application\nName=Gone\nExec=not-installed-anywhere\n")
	write(t, dir, "tryexec.desktop", "[Desktop Entry]\nType=Application\nName=Try\nTryExec=not-installed-anywhere\nExec=mytool\n")
	write(t, dir, "hidden.desktop", "[Desktop Entry]\nType=Application\nName=Hidden\nHidden=true\nExec=mytool\n")
	write(t, dir, "link.desktop", "[Desktop Entry]\nType=Link\nName=Example\nURL=https://example.com\n")
	write(t, dir, "nodisplay.desktop", "[Desktop Entry]\nType=Application\nName=NoDisplay\nNoDisplay=true\nExec=mytool\n")
	return dir, tool
}

func TestAllAppsAppliesGioLoadRules(t *testing.T) {
	dir, _ := appsFixture(t)
	var ids []string
	for _, a := range AllApps([]string{dir}) {
		ids = append(ids, a.ID)
	}
	want := map[string]bool{"mytool.desktop": true, "kde-sub.desktop": true, "nodisplay.desktop": true}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v", ids)
	}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected app %s (hidden, missing Exec/TryExec, and links are not apps)", id)
		}
	}
}

func TestAnEarlierDirWinsADesktopID(t *testing.T) {
	dir, _ := appsFixture(t)
	user := t.TempDir()
	write(t, user, "mytool.desktop", "[Desktop Entry]\nType=Application\nName=User Tool\nExec=mytool\n")
	a, ok := FindApp([]string{user, dir}, "mytool.desktop")
	if !ok || a.DisplayName() != "User Tool" {
		t.Errorf("found %q", a.DisplayName())
	}
	// A Hidden entry in the earlier dir still claims the id.
	write(t, user, "mytool.desktop", "[Desktop Entry]\nType=Application\nName=U\nHidden=true\nExec=mytool\n")
	if _, ok := FindApp([]string{user, dir}, "mytool.desktop"); ok {
		t.Error("a Hidden override must mask the system entry")
	}
}

func TestShouldShowAndShownIn(t *testing.T) {
	dir, _ := appsFixture(t)
	nd, _ := FindApp([]string{dir}, "nodisplay.desktop")
	if nd.ShouldShow() {
		t.Error("NoDisplay must not show")
	}
	kf, _ := ParseKeyFile([]byte("[Desktop Entry]\nOnlyShowIn=GNOME;\nNotShowIn=KDE;\n"))
	a := App{file: kf}
	if !a.ShownIn([]string{"GNOME"}) || a.ShownIn([]string{"KDE"}) || a.ShownIn([]string{"sway"}) {
		t.Error("OnlyShowIn/NotShowIn")
	}
	kf, _ = ParseKeyFile([]byte("[Desktop Entry]\nNotShowIn=KDE;\n"))
	a = App{file: kf}
	if !a.ShownIn([]string{"sway"}) || a.ShownIn([]string{"KDE"}) {
		t.Error("NotShowIn only")
	}
}

func TestExpandExecFieldCodes(t *testing.T) {
	dir, _ := appsFixture(t)
	a, _ := FindApp([]string{dir}, "mytool.desktop")
	for execLine, want := range map[string][]string{
		"mytool %U":              {"mytool"},
		"mytool %i --name %c":    {"mytool", "--icon", "mytool", "--name", "My Tool"},
		"mytool 100%% --file=%f": {"mytool", "100%", "--file="},
		"mytool %k":              {"mytool", a.Path},
		"'my tool' \"a b\"":      {"my tool", "a b"},
	} {
		got, err := a.ExpandExec(execLine)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("ExpandExec(%q) = %q %v, want %q", execLine, got, err, want)
		}
	}
	if _, err := a.ExpandExec("mytool 'unbalanced"); err == nil {
		t.Error("an unparseable Exec must be an error")
	}
}

// Files given to an Exec go where its codes say: a list code takes
// them all, one argument each, and a word around it keeps its text at
// either end; a single code, or none, takes the first alone, the rest
// left for another launch. A path code given a remote URI is an error
// for every target the launch would take.
func TestExpandExecOverFiles(t *testing.T) {
	dir, _ := appsFixture(t)
	a, _ := FindApp([]string{dir}, "mytool.desktop")
	targets := func(uris ...string) []launchTarget {
		out := make([]launchTarget, len(uris))
		for i, u := range uris {
			out[i] = newLaunchTarget(u)
		}
		return out
	}
	for _, tc := range []struct {
		exec string
		uris []string
		want []string
		took int
	}{
		{"app %F", []string{"file:///a", "file:///b%20c"}, []string{"app", "/a", "/b c"}, 2},
		{"app --files=%F. end", []string{"file:///a", "file:///b", "file:///c"}, []string{"app", "--files=/a", "/b", "/c.", "end"}, 3},
		{"app %U", []string{"file:///a", "https://x/y"}, []string{"app", "file:///a", "https://x/y"}, 2},
		{"app %f", []string{"file:///a", "file:///b"}, []string{"app", "/a"}, 1},
		{"app --uri=%u", []string{"https://x/y", "file:///b"}, []string{"app", "--uri=https://x/y"}, 1},
		{"app", []string{"file:///a", "file:///b"}, []string{"app", "/a"}, 1},
		{"app %U", nil, []string{"app"}, 0},
	} {
		got, took, err := a.expandExec(tc.exec, targets(tc.uris...))
		if err != nil || !slices.Equal(got, tc.want) || took != tc.took {
			t.Errorf("expandExec(%q, %q) = %q took %d, %v; want %q took %d", tc.exec, tc.uris, got, took, err, tc.want, tc.took)
		}
	}
	for _, tc := range []struct {
		exec string
		uris []string
		took int
	}{
		{"app %f", []string{"https://x/y", "file:///b"}, 1},
		{"app %F", []string{"file:///a", "https://x/y", "file:///b"}, 3},
		{"app", []string{"sftp://h/a"}, 1},
	} {
		if got, took, err := a.expandExec(tc.exec, targets(tc.uris...)); err == nil || took != tc.took || got != nil {
			t.Errorf("expandExec(%q, %q) = %q took %d, %v; want an error taking %d", tc.exec, tc.uris, got, took, err, tc.took)
		}
	}
}

// OpenAll runs an app whose Exec takes a list once with every file,
// and one that takes a single file once for each; a file a launch
// cannot take fails that launch alone, named in the error.
func TestOpenAllRunsOnceForAListAndOnceAFileOtherwise(t *testing.T) {
	dbustest.Session(t)
	bin, out := t.TempDir(), t.TempDir()
	fdtest.WriteExecutable(t, filepath.Join(bin, "rectool"), []byte("#!/bin/sh\nfor a; do printf '[%s]' \"$a\"; done > \""+out+"/$$\"\n"))
	t.Setenv("PATH", bin+":/bin:/usr/bin:"+os.Getenv("PATH"))
	dir := t.TempDir()
	write(t, dir, "list.desktop", "[Desktop Entry]\nType=Application\nName=List\nExec=rectool %F\n")
	write(t, dir, "single.desktop", "[Desktop Entry]\nType=Application\nName=Single\nExec=rectool %f\n")
	runs := func(want int) []string {
		t.Helper()
		var lines []string
		waittest.Until(t, func() bool {
			des, err := os.ReadDir(out)
			if err != nil || len(des) != want {
				return false
			}
			lines = lines[:0]
			for _, de := range des {
				data, err := os.ReadFile(filepath.Join(out, de.Name()))
				if err != nil || len(data) == 0 {
					return false
				}
				lines = append(lines, string(data))
			}
			return true
		}, "the launched app to record its arguments")
		slices.Sort(lines)
		for _, de := range must(os.ReadDir(out)) {
			if err := os.Remove(filepath.Join(out, de.Name())); err != nil {
				t.Fatal(err)
			}
		}
		return lines
	}
	uris := []string{"file:///tmp/a", "file:///tmp/b%20c"}
	list, _ := FindApp([]string{dir}, "list.desktop")
	if err := list.OpenAll(uris, LaunchContext{}); err != nil {
		t.Fatal(err)
	}
	if got := runs(1); !slices.Equal(got, []string{"[/tmp/a][/tmp/b c]"}) {
		t.Errorf("a list Exec ran %q, want one run with both files", got)
	}
	single, _ := FindApp([]string{dir}, "single.desktop")
	err := single.OpenAll(append(uris, "https://example.com/x"), LaunchContext{})
	if err == nil || !strings.Contains(err.Error(), "https://example.com/x") {
		t.Errorf("a single-file Exec given a remote URI = %v, want an error naming it", err)
	}
	if got := runs(2); !slices.Equal(got, []string{"[/tmp/a]", "[/tmp/b c]"}) {
		t.Errorf("a single-file Exec ran %q, want one run per local file", got)
	}
	if err := single.OpenAll(nil, LaunchContext{}); err == nil {
		t.Error("opening nothing ran the app")
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestActions(t *testing.T) {
	dir, _ := appsFixture(t)
	a, _ := FindApp([]string{dir}, "mytool.desktop")
	if got := a.Actions(); !reflect.DeepEqual(got, []string{"new"}) {
		t.Errorf("actions = %q (an action without a group is dropped)", got)
	}
	if a.ActionName("new") != "New Window" {
		t.Errorf("action name = %q", a.ActionName("new"))
	}
	if err := a.LaunchAction("ghost"); err == nil {
		t.Error("an unknown action must be an error")
	}
}

func TestLinks(t *testing.T) {
	dir, _ := appsFixture(t)
	links := Links([]string{dir})
	if len(links) != 1 || links[0].URL != "https://example.com" || links[0].ID != "link.desktop" {
		t.Errorf("links = %+v", links)
	}
}

func TestLaunchRunsTheExecDetached(t *testing.T) {
	dbustest.Session(t)
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(bin, "marktool")
	fdtest.WriteExecutable(t, script, []byte("#!/bin/sh\necho \"$GIO_LAUNCHED_DESKTOP_FILE\" > \""+marker+"\"\n"))
	t.Setenv("PATH", bin+":/bin:/usr/bin:"+os.Getenv("PATH"))
	dir := t.TempDir()
	path := write(t, dir, "mark.desktop", "[Desktop Entry]\nType=Application\nName=Mark\nExec=marktool %u\n")
	a, ok := FindApp([]string{dir}, "mark.desktop")
	if !ok {
		t.Fatal("fixture app not found")
	}
	if err := a.Launch(); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && string(data) == path+"\n"
	}, "the launched app to record its desktop file")
}

// LaunchArgs runs the Exec with its file codes dropped and the caller's
// arguments after it, raised by the token, each argument one word; a
// terminal entry with no terminal to run in is refused, never run bare.
func TestLaunchArgsAppendsArgumentsToTheExec(t *testing.T) {
	dbustest.Session(t)
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	fdtest.WriteExecutable(t, filepath.Join(bin, "argtool"), []byte("#!/bin/sh\nfor a; do printf '[%s]' \"$a\"; done > \""+marker+"\"\necho \"|$XDG_ACTIVATION_TOKEN\" >> \""+marker+"\"\n"))
	t.Setenv("PATH", bin+":/bin:/usr/bin:"+os.Getenv("PATH"))
	dir := t.TempDir()
	write(t, dir, "arg.desktop", "[Desktop Entry]\nType=Application\nName=Arg\nExec=argtool --name x %U\n")
	write(t, dir, "argterm.desktop", "[Desktop Entry]\nType=Application\nName=ArgTerm\nTerminal=true\nExec=argtool %u\n")
	a, ok := FindApp([]string{dir}, "arg.desktop")
	if !ok {
		t.Fatal("fixture app not found")
	}
	if err := a.LaunchArgs([]string{"-compose", "to='a b',x=1"}, LaunchContext{ActivationToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	want := "[--name][x][-compose][to='a b',x=1]|tok\n"
	waittest.Until(t, func() bool {
		data, err := os.ReadFile(marker)
		return err == nil && string(data) == want
	}, "the launched app to record its arguments")

	term, ok := FindApp([]string{dir}, "argterm.desktop")
	if !ok {
		t.Fatal("terminal fixture app not found")
	}
	if err := term.LaunchArgs([]string{"-x"}, LaunchContext{}); err == nil {
		t.Error("a terminal entry ran with no terminal to run in")
	}
}

// A portal request names its app by the entry's name and icon; an
// unknown id stands for itself, and no id is a host app.
func TestDescribe(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "applications/org.example.Viewer.desktop", "[Desktop Entry]\nType=Application\nName=Viewer\nIcon=viewer-icon\nExec=true\n")
	dirs := []string{filepath.Join(dir, "applications")}
	if name, icon := Describe(dirs, "org.example.Viewer"); name != "Viewer" || icon != "viewer-icon" {
		t.Errorf("known app = %q %q", name, icon)
	}
	if name, icon := Describe(dirs, "org.unknown.App"); name != "org.unknown.App" || icon != "" {
		t.Errorf("unknown app = %q %q", name, icon)
	}
	if name, _ := Describe(dirs, ""); name != "An application" {
		t.Errorf("host app = %q", name)
	}
}

// The portal's application id is the desktop id without ".desktop",
// and maps back to the same desktop id.
func TestAppIDRoundTrip(t *testing.T) {
	a := App{ID: "org.example.Viewer.desktop"}
	if got := a.AppID(); got != "org.example.Viewer" {
		t.Errorf("AppID = %q", got)
	}
	if got := FileID(a.AppID()); got != a.ID {
		t.Errorf("FileID = %q, want %q", got, a.ID)
	}
	if got := (App{ID: "desktop.desktop"}).AppID(); got != "desktop" {
		t.Errorf("AppID strips more than the suffix: %q", got)
	}
}

// FindApps answers the ids in the order asked, the earlier dir winning,
// and skips the ids with no loadable entry.
func TestFindAppsKeepsTheOrderAsked(t *testing.T) {
	dir, _ := appsFixture(t)
	user := t.TempDir()
	write(t, user, "mytool.desktop", "[Desktop Entry]\nType=Application\nName=User Tool\nExec=mytool\n")
	write(t, user, "gone.desktop", "[Desktop Entry]\nType=Application\nName=Gone\nHidden=true\nExec=mytool\n")
	apps := FindApps([]string{user, dir}, []string{"nodisplay.desktop", "missing.desktop", "gone.desktop", "mytool.desktop"})
	var got []string
	for _, a := range apps {
		got = append(got, a.ID+"="+a.DisplayName())
	}
	if want := []string{"nodisplay.desktop=NoDisplay", "mytool.desktop=User Tool"}; !reflect.DeepEqual(got, want) {
		t.Errorf("FindApps = %q, want %q", got, want)
	}
}

func TestPathProgramsFindsWhatAPathScanFinds(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), nil, mode); err != nil {
			t.Fatal(err)
		}
	}
	write("runs", 0o755)
	write("notexec", 0o644)
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "absent")+":"+dir)
	onPath := pathPrograms()
	for program, want := range map[string]bool{"runs": true, "notexec": false, "adir": false, "absent": false} {
		if got := onPath(program); got != want {
			t.Errorf("listed lookup of %q = %v, want %v", program, got, want)
		}
		if got := findOnPath(program); got != want {
			t.Errorf("scan of %q = %v, want %v", program, got, want)
		}
	}
}

func TestKeyFileLinesEndAsBufioScansThem(t *testing.T) {
	for _, data := range []string{
		"[Desktop Entry]\nName=Files\nExec=nautilus\n",
		"[Desktop Entry]\r\nName=Files\r\nExec=nautilus\r\n",
		"[Desktop Entry]\nName=Files\nExec=nautilus",
	} {
		kf, err := ParseKeyFile([]byte(data))
		if err != nil {
			t.Fatalf("%q: %v", data, err)
		}
		for key, want := range map[string]string{"Name": "Files", "Exec": "nautilus"} {
			if got, ok := kf.String(DesktopGroup, key); !ok || got != want {
				t.Errorf("%q: %s is %q, %v; want %q", data, key, got, ok, want)
			}
		}
	}
	if _, err := ParseKeyFile([]byte("[Desktop Entry]\r\nName\r\n")); !errors.Is(err, ErrMalformed) {
		t.Errorf("a CRLF line without = parsed: %v", err)
	}
}
