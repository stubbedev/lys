package filechooser

import (
	"testing"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

func TestMatchesFilterHandlesGlobsMIMEAndBounds(t *testing.T) {
	db := browsertest.MIME(t)
	filters := []Filter{
		{"Images", []Rule{{RuleGlob, "*.png"}, {RuleGlob, "*.jpg"}}},
		{"Text", []Rule{{RuleGlob, "*.txt"}}},
	}
	check := func(name string, fs []Filter, active int, want bool) {
		t.Helper()
		if got := MatchesFilter(db, name, fs, active); got != want {
			t.Errorf("MatchesFilter(%q, %d) = %v, want %v", name, active, got, want)
		}
	}
	check("a.png", filters, 0, true)
	check("a.jpg", filters, 0, true)
	check("a.txt", filters, 0, false)
	check("a.txt", filters, 1, true)
	check("a.png", filters, 1, false)
	check("anything", nil, 0, true)
	check("x.bin", []Filter{{"All", []Rule{{RuleGlob, "*"}}}}, 0, true)
	exact := []Filter{{"Make", []Rule{{RuleGlob, "Makefile"}}}}
	check("Makefile", exact, 0, true)
	check("makefile", exact, 0, true)
	check("Makefile.old", exact, 0, false)
	mimeOnly := []Filter{{"Img", []Rule{{RuleMIME, "image/png"}}}}
	check("photo.png", mimeOnly, 0, true)
	check("notes.txt", mimeOnly, 0, false)
	images := []Filter{{"Images", []Rule{{RuleMIME, "image/*"}}}}
	check("a.png", images, 0, true)
	check("a.jpg", images, 0, true)
	check("a.txt", images, 0, false)
	// A MIME rule takes subtypes: svg is xml is text.
	check("drawing.svg", []Filter{{"Text", []Rule{{RuleMIME, "text/plain"}}}}, 0, true)
	check("x.bin", []Filter{{"Any", nil}}, 0, true)
	check("x", filters, 99, true)
	check("x", filters, -1, true)
}

func TestGTK4SuffixFiltersMatchInAnyCase(t *testing.T) {
	db := browsertest.MIME(t)
	gtk4 := []Filter{{"Images", []Rule{
		{RuleGlob, "*.[pP][nN][gG]"},
		{RuleGlob, "*.[jJ][pP][gG]"},
		{RuleGlob, "*.gif"},
		{RuleMIME, "image/webp"},
	}}}
	for _, name := range []string{"photo.png", "Photo.PNG", "photo.PnG", "shot.jpg", "SHOT.JPG", "anim.gif", "ANIM.GIF", "sticker.webp"} {
		if !MatchesFilter(db, name, gtk4, 0) {
			t.Errorf("%s should match", name)
		}
	}
	for _, name := range []string{"photo.txt", "notes.md", "png", "photo.png.bak"} {
		if MatchesFilter(db, name, gtk4, 0) {
			t.Errorf("%s should not match", name)
		}
	}
}

func TestGlobMatcherHandlesWildcardsClassesAndNegation(t *testing.T) {
	for _, c := range []struct {
		name, glob string
		want       bool
	}{
		{"archive.tar.gz", "*.tar.*", true},
		{"archive.zip", "*.tar.*", false},
		{"image_1.png", "image_?.png", true},
		{"image_10.png", "image_?.png", false},
		{"1file", "[!a-z]*", true},
		{"afile", "[!a-z]*", false},
		{"report_2024.pdf", "report_*.pdf", true},
		// Unparseable degrades to an exact, case-insensitive compare.
		{"[oops", "[oops", true},
		{"[OOPS", "[oops", true},
		{"other", "[oops", false},
	} {
		if got := matchGlob(c.name, c.glob); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.name, c.glob, got, c.want)
		}
	}
}

func TestMimeExtCoversCommonTypes(t *testing.T) {
	for mimeType, want := range map[string]string{
		"image/png": "png", "image/jpeg": "jpg", "image/svg+xml": "svg", "image/gif": "gif",
		"image/webp": "webp", "video/mp4": "mp4", "video/x-matroska": "mkv",
		"application/pdf": "pdf", "application/zip": "zip", "application/json": "json",
		"text/plain": "txt", "text/csv": "csv", "text/html": "html",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
		"audio/mpeg": "mp3", "audio/flac": "flac", "audio/aac": "aac", "video/quicktime": "mov",
	} {
		if got := mimeExt(mimeType); got != want {
			t.Errorf("mimeExt(%s) = %q, want %q", mimeType, got, want)
		}
	}
	for _, unknown := range []string{"image/*", "application/x-made-up"} {
		if got := mimeExt(unknown); got != "" {
			t.Errorf("mimeExt(%s) = %q, want none", unknown, got)
		}
	}
}

func TestFilterLabelPrefersNameFallsBackToPatterns(t *testing.T) {
	for _, c := range []struct {
		f    Filter
		want string
	}{
		{Filter{"Images", []Rule{{RuleGlob, "*.png"}, {RuleGlob, "*.[Jj][Pp][Gg]"}}}, "Images"},
		{Filter{"", []Rule{{RuleGlob, "*.png"}, {RuleMIME, "image/png"}}}, "*.png"},
		{Filter{" ", []Rule{{RuleMIME, "image/jpeg"}}}, "*.jpg"},
		{Filter{"", []Rule{{RuleMIME, "image/*"}}}, "image/*"},
	} {
		if got := FilterLabel(c.f); got != c.want {
			t.Errorf("FilterLabel(%+v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestWithAllFilesAddsTheEscapeHatchOnce(t *testing.T) {
	images := []Filter{{"Images", []Rule{{RuleGlob, "*.png"}}}}
	got := withAllFiles(images)
	if len(got) != 2 || got[1].Name != "All Files" {
		t.Fatalf("restrictive filters = %+v, want All Files appended", got)
	}
	if len(images) != 1 {
		t.Error("appending mutated the request's filters")
	}
	for _, catchAll := range []string{"*", "*.*"} {
		fs := []Filter{{"Any", []Rule{{RuleGlob, catchAll}}}}
		if got := withAllFiles(fs); len(got) != 1 {
			t.Errorf("a %q filter got All Files appended", catchAll)
		}
	}
	if got := withAllFiles(nil); len(got) != 0 {
		t.Errorf("no filters = %+v, want still none", got)
	}
}

// The appended All Files must not land in the caller's spare capacity,
// where the caller's next append would overwrite it.
func TestWithAllFilesDoesNotAliasTheRequest(t *testing.T) {
	request := make([]Filter, 1, 4)
	request[0] = Filter{"Images", []Rule{{RuleGlob, "*.png"}}}
	got := withAllFiles(request)
	_ = append(request, Filter{Name: "Clobber"})
	if got[1].Name != "All Files" {
		t.Errorf("All Files became %q through the request's backing array", got[1].Name)
	}
}

// A filter names an extension when it is one *.ext glob (case classes
// read back) or one MIME type with a canonical one.
func TestFilterExt(t *testing.T) {
	for _, tc := range []struct {
		f    Filter
		want string
	}{
		{Filter{Rules: []Rule{{RuleGlob, "*.txt"}}}, "txt"},
		{Filter{Rules: []Rule{{RuleGlob, "*.[pP][nN][gG]"}}}, "png"},
		{Filter{Rules: []Rule{{RuleMIME, "application/pdf"}}}, "pdf"},
		{Filter{Rules: []Rule{{RuleGlob, "*"}}}, ""},
		{Filter{Rules: []Rule{{RuleGlob, "*.tar.*"}}}, ""},
		{Filter{Rules: []Rule{{RuleGlob, "*.png"}, {RuleGlob, "*.jpg"}}}, ""},
		{Filter{Rules: []Rule{{RuleMIME, "image/*"}}}, ""},
	} {
		if got := filterExt(tc.f); got != tc.want {
			t.Errorf("filterExt(%v) = %q, want %q", tc.f.Rules, got, tc.want)
		}
	}
	if got := withExtension("notes.md", Filter{Rules: []Rule{{RuleGlob, "*.txt"}}}); got != "notes.txt" {
		t.Errorf("withExtension = %q", got)
	}
	if got := withExtension("", Filter{Rules: []Rule{{RuleGlob, "*.txt"}}}); got != "" {
		t.Errorf("an empty name took %q", got)
	}
}
