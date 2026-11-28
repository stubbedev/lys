package recentfiles

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/xdg"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recently-used.xbel")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func paths(entries []Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Path
	}
	return out
}

// Entries are ordered by the time their stamps name, not their text: a
// stamp in another zone that reads later can be earlier. A bookmark with
// no modified time is ordered by its visited one, and one with neither
// comes last; remote and non-file URIs drop out, and paths decode.
func TestLoadIsNewestFirstByTime(t *testing.T) {
	path := write(t, `<?xml version="1.0"?><xbel version="1.0">
<bookmark href="file:///tmp/earlier" modified="2026-01-01T10:00:00+05:00"/>
<bookmark href="file:///tmp/later" modified="2026-01-01T06:00:00Z"><info><metadata owner="x"><app name="gedit"/></metadata></info></bookmark>
<bookmark href="file:///tmp/visited%20only" visited="2026-01-01T07:00:00Z"/>
<bookmark href="file:///tmp/undated"/>
<bookmark href="sftp://host/remote" modified="2026-12-01T00:00:00Z"/>
<bookmark href="file://otherhost/elsewhere" modified="2026-12-01T00:00:00Z"/>
</xbel>`)
	entries, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := paths(entries), []string{"/tmp/visited only", "/tmp/later", "/tmp/earlier", "/tmp/undated"}; !slices.Equal(got, want) {
		t.Errorf("order = %q, want %q", got, want)
	}
	if entries[1].App != "gedit" || !entries[1].When.Equal(time.Date(2026, 1, 1, 6, 0, 0, 0, time.UTC)) || !entries[3].When.IsZero() {
		t.Errorf("entries = %+v", entries)
	}
}

func TestLoadRefusesWhatIsNotAListAndAcceptsNone(t *testing.T) {
	if entries, err := Load(filepath.Join(t.TempDir(), "none.xbel")); err != nil || entries != nil {
		t.Errorf("a missing list = %v, %v, want nothing and no error", entries, err)
	}
	if _, err := Load(write(t, `<xbel version="1.0"><bookmark`)); err == nil {
		t.Error("a broken list loaded without an error")
	}
	huge := write(t, `<xbel version="1.0">`+strings.Repeat(`<bookmark href="file:///tmp/x"/>`, maxBytes/30)+`</xbel>`)
	if _, err := Load(huge); err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("a list over the cap loaded with %v", err)
	}
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("a directory loaded as a list")
	}
}

func TestMarshalRoundTripsThroughLoad(t *testing.T) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	in := []Entry{
		{Path: "/tmp/new file #1.txt", App: "lys-files", When: at.Add(time.Hour)},
		{Path: "/tmp/old", App: "lys-ui", When: at},
	}
	data, err := Marshal(in, "lys-ui")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "<?xml") || !strings.Contains(string(data), `href="file:///tmp/new%20file%20%231.txt"`) {
		t.Errorf("document = %s", data)
	}
	out, err := Load(write(t, string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(out, in) {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
	empty, err := Marshal(nil, "lys-ui")
	if err != nil || !strings.Contains(string(empty), "<xbel") {
		t.Errorf("an empty list = %s, %v", empty, err)
	}
}

func TestDefaultPathIsAbsoluteOrAnError(t *testing.T) {
	env := func(vars map[string]string) xdg.Env {
		return func(name string) string { return vars[name] }
	}
	for want, vars := range map[string]map[string]string{
		"/data/recently-used.xbel":                 {"XDG_DATA_HOME": "/data", "HOME": "/home/me"},
		"/home/me/.local/share/recently-used.xbel": {"XDG_DATA_HOME": "data", "HOME": "/home/me"},
	} {
		if got, err := DefaultPath(env(vars)); err != nil || got != want {
			t.Errorf("%v: %q, %v, want %q", vars, got, err, want)
		}
	}
	for _, vars := range []map[string]string{nil, {"XDG_DATA_HOME": "data", "HOME": "me"}} {
		if got, err := DefaultPath(env(vars)); !errors.Is(err, ErrNoDataHome) || got != "" {
			t.Errorf("%v: %q, %v, want ErrNoDataHome", vars, got, err)
		}
	}
}
