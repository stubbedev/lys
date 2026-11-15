package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/service/wallpaper/extract"
	"github.com/stubbedev/lys/lib/dbustest"

	"github.com/stubbedev/lys/shell/service/wallpaper"
)

var updateWallpaperGolden = flag.Bool("update-wallpaper-golden", false, "rewrite testdata/wallpaper-cli.golden from the CLI's output")

// wallpaperScenarios run against a fresh daemon each: DP-1 and DP-2
// registered, @DIR@ holding a.png and b.png, @EMPTY@ holding no image.
// "no-daemon" scenarios run with nothing owning the name.
var wallpaperScenarios = []struct {
	name     string
	noDaemon bool
	steps    [][]string
}{
	{name: "info-fresh", steps: [][]string{{"info"}, {"info", "--monitor", "DP-1"}}},
	{name: "set-all", steps: [][]string{{"set", "@DIR@/a.png"}, {"info", "--monitor", "DP-1"}, {"info", "--monitor", "DP-2"}}},
	{name: "set-monitor-fit", steps: [][]string{
		{"set", "@DIR@/b.png", "--fit", "center", "--monitor", "DP-2"},
		{"info", "--monitor", "DP-2"},
		{"info", "--monitor", "DP-1"},
		{"info"},
	}},
	{name: "set-tile", steps: [][]string{{"set", "@DIR@/a.png", "-f", "tile"}}},
	{name: "set-missing", steps: [][]string{{"set", "/nonexistent/x.png"}, {"set", "/nonexistent/x.png", "--monitor", "DP-1"}}},
	{name: "cycle", steps: [][]string{
		{"cycle", "@DIR@"},
		{"info", "--monitor", "DP-1"},
		{"next"},
		{"info", "--monitor", "DP-2"},
		{"previous"},
		{"info", "--monitor", "DP-1"},
		{"stop"},
		{"info"},
	}},
	// Shuffle picks random images, so only the start message is compared.
	{name: "cycle-flags", steps: [][]string{{"cycle", "@DIR@", "-i", "60", "-m", "shuffle"}, {"cycle", "@DIR@", "--interval=5", "--mode=sequential"}}},
	{name: "cycle-missing", steps: [][]string{{"cycle", "/nonexistent/dir"}}},
	{name: "cycle-empty", steps: [][]string{{"cycle", "@EMPTY@"}}},
	{name: "next-without-cycle", steps: [][]string{{"next"}, {"previous"}, {"stop"}}},
	{name: "theming", steps: [][]string{{"theming-monitor", "DP-1"}, {"theming-monitor", ""}}},
	{name: "not-running", noDaemon: true, steps: [][]string{{"info"}, {"stop"}, {"set", "@DIR@/a.png"}}},
}

func runScenario(t *testing.T, sc struct {
	name     string
	noDaemon bool
	steps    [][]string
},
) string {
	t.Helper()
	dbustest.Session(t)
	dir, empty := t.TempDir(), t.TempDir()
	for _, n := range []string{"a.png", "b.png"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if !sc.noDaemon {
		conn := dbustest.SessionConn(t)
		svc := wallpaper.New(wallpaper.Options{Extractor: extract.Config{Tool: extract.None}})
		svc.RegisterMonitor("DP-1")
		svc.RegisterMonitor("DP-2")
		release, err := wallpaper.Export(conn, svc)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(release)
	}
	var log strings.Builder
	for _, step := range sc.steps {
		args := make([]string, len(step))
		for i, a := range step {
			args[i] = strings.NewReplacer("@DIR@", dir, "@EMPTY@", empty).Replace(a)
		}
		fmt.Fprintf(&log, "$ wallpaper %q\n", step)
		stdout, stderr, code := runCaptured(t, false, append([]string{"wallpaper"}, args...)...)
		fmt.Fprintf(&log, "%s[stderr] %s[exit %d]\n", stdout, stderr, code)
	}
	return strings.NewReplacer(dir, "@DIR@", empty, "@EMPTY@").Replace(log.String())
}

// TestWallpaperCLIGolden replays the scenarios through the CLI and
// compares stdout, the error line, and the exit code with the golden.
func TestWallpaperCLIGolden(t *testing.T) {
	golden := filepath.Join("testdata", "wallpaper-cli.golden")
	var all strings.Builder
	for _, sc := range wallpaperScenarios {
		fmt.Fprintf(&all, "### %s\n", sc.name)
		t.Run(sc.name, func(t *testing.T) { all.WriteString(runScenario(t, sc)) })
	}
	if *updateWallpaperGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(all.String()), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if all.String() != string(want) {
		t.Errorf("wallpaper CLI output differs from the golden:\n--- got\n%s\n--- want\n%s", all.String(), want)
	}
}

func TestWallpaperCLIRejectsBadArgsBeforeDialing(t *testing.T) {
	// No bus at all: every case must fail in parsing (exit 2), not
	// connecting.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/bus")
	for _, args := range [][]string{
		{},
		{"bogus"},
		{"set"},
		{"set", "a.png", "--fit", "cover"},
		{"set", "a.png", "--fit"},
		{"set", "a.png", "--bogus", "x"},
		{"cycle", "dir", "--interval", "-5"},
		{"cycle", "dir", "--mode", "random"},
		{"stop", "extra"},
		{"theming-monitor"},
	} {
		_, stderr, code := runCaptured(t, false, append([]string{"wallpaper"}, args...)...)
		if code != 2 || strings.Contains(stderr, "D-Bus") {
			t.Errorf("%q: code %d stderr %q, want a parse error", args, code, stderr)
		}
	}
}
