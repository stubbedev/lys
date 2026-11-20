package settings

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedLock(t *testing.T, p *settingsPage) (preview *livePreview, labels []string) {
	t.Helper()
	preview = onlyPreview(t, p)
	if len(treetest.WithClass(preview, "lock-window")) != 1 {
		t.Fatalf("the lock page previews %v instead of one lock screen", treetest.Labels(preview))
	}
	return preview, treetest.Labels(preview)
}

func lockClock(t *testing.T, preview *livePreview) *widget.Label {
	t.Helper()
	found := treetest.WithClass(preview, "lock-clock")
	if len(found) != 1 {
		t.Fatalf("the lock preview holds %d clocks", len(found))
	}
	return found[0].(*widget.Label)
}

func writePNG(t *testing.T, name string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 40))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	path := filepath.Join(t.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(png.Encode(f, img), f.Close()); err != nil {
		t.Fatal(err)
	}
	return path
}

func previewedBackdrop(t *testing.T, k *kit, preview *livePreview) render.ImageID {
	t.Helper()
	if pics := treetest.All[*widget.Image](preview); len(pics) != 1 {
		t.Fatalf("the lock preview draws %d images, want the backdrop", len(pics))
	}
	held := k.lockBackdrop().showing.backdrop
	if held == nil {
		t.Fatal("the lock preview draws an image while it holds no backdrop")
	}
	return held.Image.ID
}

func TestTheLockPagePreviewsTheLockScreenItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, lockPage(nil))
	preview, labels := previewedLock(t, p)
	cfg := config.Defaults().Login
	clock, date := cfg.ClockFormat.Layout().Format(lockPreviewMoment), cfg.DateFormat.Layout().Format(lockPreviewMoment)
	for _, want := range []string{clock, date, locale.T().Get("settings-lock-preview-account")} {
		if !slices.Contains(labels, want) {
			t.Errorf("the lock preview shows %v, want %q", labels, want)
		}
	}
	if !lockClock(t, preview).Visible() {
		t.Error("the default lock preview hides its clock")
	}
	if len(treetest.WithClass(preview, "lock-secret")) != 1 {
		t.Error("the lock preview has no password field")
	}
	if len(treetest.WithClass(preview, "lock-bg")) != 1 || len(treetest.All[*widget.Image](preview)) != 0 {
		t.Error("the default colour background previews no colour fill")
	}

	set(t, k, p, "login.clock-format", "%H.%M")
	_, labels = previewedLock(t, p)
	if !slices.Contains(labels, "09.41") {
		t.Errorf("clock-format %%H.%%M previews %v, want 09.41", labels)
	}
	set(t, k, p, "login.show-clock", false)
	preview, _ = previewedLock(t, p)
	if lockClock(t, preview).Visible() {
		t.Error("show-clock off still previews the clock")
	}
}

func TestTheLockPreviewDrawsTheImageBlurredAsTheDraftSays(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, lockPage(nil))
	set(t, k, p, "login.background-image", writePNG(t, "lock.png"))
	set(t, k, p, "login.background-mode", string(config.BackgroundImage))
	preview, _ := previewedLock(t, p)
	first := previewedBackdrop(t, k, preview)
	if len(treetest.WithClass(preview, "lock-bg-scrim")) != 1 {
		t.Error("the image previews without the lock screen's scrim")
	}
	if len(treetest.WithClass(preview, "settings-preview-state")) != 0 {
		t.Errorf("a readable image previews with a note: %v", treetest.Labels(preview))
	}
	p.refresh()
	preview, _ = previewedLock(t, p)
	if again := previewedBackdrop(t, k, preview); again != first {
		t.Error("an unchanged draft loaded its background again")
	}
	set(t, k, p, "login.blur", int64(3))
	preview, _ = previewedLock(t, p)
	if blurred := previewedBackdrop(t, k, preview); blurred == first {
		t.Error("a new blur previews the old backdrop")
	}
}

func TestALockBackgroundThatCannotBeShownPreviewsTheColourAndWhy(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, lockPage(nil))
	missing := filepath.Join(t.TempDir(), "gone.png")
	set(t, k, p, "login.background-image", missing)
	set(t, k, p, "login.background-mode", string(config.BackgroundImage))
	preview, labels := previewedLock(t, p)
	if len(treetest.All[*widget.Image](preview)) != 0 || len(treetest.WithClass(preview, "lock-bg")) != 1 {
		t.Error("an unreadable image does not preview the colour fill")
	}
	_, err := os.Open(missing)
	want := locale.T().Get("settings-lock-preview-unreadable", i18n.Str("path", missing), i18n.Str("reason", "background "+missing+" unreadable: "+err.Error()))
	if !slices.Contains(labels, want) {
		t.Errorf("an unreadable image previews %v, want %q", labels, want)
	}

	set(t, k, p, "login.background-mode", string(config.BackgroundWallpaper))
	set(t, k, p, "wallpaper.wallpaper", "")
	_, labels = previewedLock(t, p)
	if want := locale.T().Get("settings-lock-preview-no-image"); !slices.Contains(labels, want) {
		t.Errorf("the wallpaper mode without a wallpaper previews %v, want %q", labels, want)
	}
}

func TestTheLockPreviewLetsGoOfABackdropTheDraftNoLongerNames(t *testing.T) {
	k := testKit(t, "")
	var forgotten []render.ImageID
	k.forgetImage = func(id render.ImageID) { forgotten = append(forgotten, id) }
	p := buildPage(k, lockPage(nil))
	set(t, k, p, "login.background-image", writePNG(t, "lock.png"))
	set(t, k, p, "login.background-mode", string(config.BackgroundImage))
	preview, _ := previewedLock(t, p)
	held := previewedBackdrop(t, k, preview)
	if len(forgotten) != 0 {
		t.Fatalf("loading the backdrop forgot %v", forgotten)
	}
	set(t, k, p, "login.background-mode", string(config.BackgroundColor))
	if !slices.Equal(forgotten, []render.ImageID{held}) {
		t.Errorf("switching to the colour forgot %v, want the held backdrop %v", forgotten, held)
	}
}
