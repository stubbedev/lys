package files

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

	"github.com/stubbedev/lys/files/internal/filemanager1"
)

func resource(t *testing.T, name string) *desktopentry.KeyFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", name))
	if err != nil {
		t.Fatal(err)
	}
	kf, err := desktopentry.ParseKeyFile(data)
	if err != nil {
		t.Fatal(err)
	}
	return kf
}

// The desktop entry makes lys-files the app for folders, launched with
// the folders' URIs and matched to the window by its app id.
func TestTheDesktopFileClaimsFolders(t *testing.T) {
	kf := resource(t, AppID+".desktop")
	if got := kf.List(desktopentry.DesktopGroup, "MimeType"); !slices.Equal(got, []string{"inode/directory"}) {
		t.Errorf("MimeType= lists %v, want inode/directory alone", got)
	}
	for key, want := range map[string]string{"StartupWMClass": AppID, "Icon": "lys-files", "Exec": "lys-files %U", "Type": "Application"} {
		if got, _ := kf.String(desktopentry.DesktopGroup, key); got != want {
			t.Errorf("%s=%q, want %q", key, got, want)
		}
	}
}

// The D-Bus activation file starts lys-files as the service that owns
// FileManager1's name.
func TestTheActivationFileStartsTheService(t *testing.T) {
	kf := resource(t, filemanager1.ServiceName+".service")
	const group = "D-BUS Service"
	if name, _ := kf.String(group, "Name"); name != filemanager1.ServiceName {
		t.Errorf("Name=%q, want %q", name, filemanager1.ServiceName)
	}
	if exec, _ := kf.String(group, "Exec"); exec != "/usr/bin/lys-files "+ServiceArg {
		t.Errorf("Exec=%q, want /usr/bin/lys-files %s", exec, ServiceArg)
	}
}

// Every icon the window shows, the browser's and the preview's
// included, is one resources/icons.txt ships, and it ships no other.
func TestEveryIconTheFileManagerShowsIsOneItShips(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "icons.txt"))
	if err != nil {
		t.Fatal(err)
	}
	listed := strings.Fields(string(data))
	repo := filepath.Join("..", "..", "..")
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	browser, err := filepath.Glob(filepath.Join(repo, "shell", "shell", "filebrowser", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	sources = append(append(sources, browser...), filepath.Join(repo, "desktop", "shell", "preview", "view.go"), filepath.Join(repo, "desktop", "mime", "mime.go"))
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
	if !named["ld-plus-symbolic"] || !named["ld-folder-symbolic"] || !named["ld-trash-2-symbolic"] {
		t.Fatalf("found %v, so the pattern or the sources are wrong", named)
	}
	for _, icon := range listed {
		if !named[icon] {
			t.Errorf("resources/icons.txt lists %s, which nothing the file manager shows names", icon)
		}
		if _, err := fs.Stat(resources.Icons, path.Join(resources.IconsDir, icon+".svg")); err != nil {
			t.Errorf("resources/icons.txt lists %s, which the bundle does not hold", icon)
		}
	}
}
