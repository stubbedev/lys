package viewer

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/resources"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/lib/media/demux"
)

func TestTheViewerClaimsEveryPictureTypeAndTheVideoOfEveryContainer(t *testing.T) {
	types := MimeTypes()
	for _, picture := range preview.PictureTypes() {
		if !slices.Contains(types, picture) {
			t.Errorf("the viewer does not claim %s, which the preview shows as a picture", picture)
		}
	}
	for base := range pictureSubtypes {
		if !slices.Contains(preview.PictureTypes(), base) {
			t.Errorf("the viewer claims subtypes of %s, which the preview does not show as a picture", base)
		}
	}
	for f := demux.Format(1); !strings.HasPrefix(f.String(), "Format("); f++ {
		if _, ok := containerTypes[f]; !ok {
			t.Errorf("demux reads %v, of which the viewer claims neither video types nor none", f)
		}
	}
	for _, mimeType := range types {
		if !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") {
			t.Errorf("the viewer claims %s, neither a picture nor a video", mimeType)
		}
	}
	for _, unclaimed := range []string{"audio/ogg", "audio/flac", "audio/mpeg", "application/pdf", "image/svg+xml", "text/plain"} {
		if slices.Contains(types, unclaimed) {
			t.Errorf("the viewer claims %s", unclaimed)
		}
	}
	if !slices.IsSorted(types) || len(slices.Compact(slices.Clone(types))) != len(types) {
		t.Errorf("the types are not sorted once each: %v", types)
	}
}

func desktopFile(t *testing.T) *desktopentry.KeyFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", AppID+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	kf, err := desktopentry.ParseKeyFile(data)
	if err != nil {
		t.Fatal(err)
	}
	return kf
}

func TestTheDesktopFileClaimsTheTypesTheViewerShows(t *testing.T) {
	kf := desktopFile(t)
	if got, want := kf.List(desktopentry.DesktopGroup, "MimeType"), MimeTypes(); !slices.Equal(got, want) {
		t.Errorf("MimeType= lists\n%v\nwant the types the viewer claims\n%v", got, want)
	}
	for key, want := range map[string]string{"StartupWMClass": AppID, "Icon": "lys-viewer", "Exec": "lys-viewer %U", "Type": "Application"} {
		if got, _ := kf.String(desktopentry.DesktopGroup, key); got != want {
			t.Errorf("%s=%q, want %q", key, got, want)
		}
	}
}

func listedIcons(t *testing.T) []string {
	t.Helper()
	list, err := os.ReadFile(filepath.Join("..", "..", "resources", "icons.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(list))
}

func TestEveryIconTheViewerShowsIsOneItShips(t *testing.T) {
	listed := listedIcons(t)
	own, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := append(own, filepath.Join("..", "..", "..", "desktop", "shell", "preview", "view.go"), filepath.Join("..", "..", "..", "desktop", "mime", "mime.go"))
	name := regexp.MustCompile(`"((?:ld|si|tb|md|cm)-[a-z0-9-]+-symbolic)"`)
	named := map[string]bool{}
	for _, f := range sources {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range name.FindAllSubmatch(data, -1) {
			icon := string(m[1])
			named[icon] = true
			if !slices.Contains(listed, icon) {
				t.Errorf("%s names %s, which resources/icons.txt does not list", f, icon)
			}
		}
	}
	if !named[trashIcon] || !named["ld-folder-symbolic"] || !named["ld-file-image-symbolic"] {
		t.Fatalf("found %v, so the pattern or the sources are wrong", named)
	}
	for _, icon := range listed {
		if !named[icon] {
			t.Errorf("resources/icons.txt lists %s, which nothing the viewer shows names", icon)
		}
		if _, err := fs.Stat(resources.Icons, path.Join(resources.IconsDir, icon+".svg")); err != nil {
			t.Errorf("resources/icons.txt lists %s, which the bundle does not hold", icon)
		}
	}
}
