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
	"github.com/stubbedev/lys/desktop/shell/lockview"
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

func backdropImages(preview *livePreview) []*widget.Image {
	var pics []*widget.Image
	for _, pic := range treetest.All[*widget.Image](preview) {
		if !pic.HasClass("lock-media-art") {
			pics = append(pics, pic)
		}
	}
	return pics
}

func previewedBackdrop(t *testing.T, k *kit, preview *livePreview) render.ImageID {
	t.Helper()
	if pics := backdropImages(preview); len(pics) != 1 {
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
	if len(treetest.WithClass(preview, "lock-bg")) != 1 || len(backdropImages(preview)) != 0 {
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
	if len(backdropImages(preview)) != 0 || len(treetest.WithClass(preview, "lock-bg")) != 1 {
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

func lockPreviewMode(t *testing.T, p *settingsPage, mode lockview.Mode) {
	t.Helper()
	found := treetest.WithClass(p, "settings-lock-preview-mode")
	if len(found) != 1 {
		t.Fatalf("the lock page holds %d preview mode switches", len(found))
	}
	for i, m := range lockPreviewModes {
		if m.mode == mode {
			pickChoice(found[0].(*choicePick), i)
			return
		}
	}
	t.Fatalf("no preview mode %v", mode)
}

func shownWithClass(t *testing.T, preview *livePreview, class string) bool {
	t.Helper()
	found := treetest.WithClass(preview, class)
	switch len(found) {
	case 0:
		return false
	case 1:
		return found[0].(interface{ Visible() bool }).Visible()
	}
	t.Fatalf("the lock preview holds %d %s", len(found), class)
	return false
}

func TestTheLockPreviewShowsTheRowsItsDraftTurnsOn(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, lockPage(nil))
	preview, labels := previewedLock(t, p)
	tr := locale.T()
	for _, want := range []string{tr.Get("settings-lock-preview-track"), tr.Get("settings-lock-preview-artist"), i18n.T("lock-battery", i18n.Str("percent", "80")), i18n.T("lock-switch-user"), i18n.T("lock-fingerprint-touch")} {
		if !slices.Contains(labels, want) {
			t.Errorf("the default lock preview shows %v, want %q", labels, want)
		}
	}
	for _, class := range []string{"lock-media", "lock-battery", "lock-switch-user", "lock-fingerprint"} {
		if !shownWithClass(t, preview, class) {
			t.Errorf("the default lock preview hides its %s", class)
		}
	}
	if shownWithClass(t, preview, "lock-secret") {
		t.Error("the fingerprint mode previews the password entry instead of the reader")
	}

	for _, c := range []struct{ key, class string }{
		{"login.lock.show-media", "lock-media"},
		{"login.lock.show-battery", "lock-battery"},
		{"login.lock.show-switch-user", "lock-switch-user"},
		{"login.lock.fingerprint", "lock-fingerprint"},
	} {
		set(t, k, p, c.key, false)
		preview, _ = previewedLock(t, p)
		if shownWithClass(t, preview, c.class) {
			t.Errorf("%s off still previews the %s", c.key, c.class)
		}
	}
	if !shownWithClass(t, preview, "lock-secret") {
		t.Error("fingerprint off previews no password entry")
	}
}

func TestTheLockPreviewDrawsTheGreeterItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, lockPage(nil))
	lockPreviewMode(t, p, lockview.Greeter)
	if got := runtimeFile(t, k); got != "" {
		t.Errorf("switching the previewed screen wrote %q", got)
	}
	preview, labels := previewedLock(t, p)
	greeter := i18n.Greeter()
	for _, want := range append(lockPreviewAccounts(), greeter.Get("greeter-sleep"), greeter.Get("greeter-restart"), greeter.Get("greeter-shutdown")) {
		if !slices.Contains(labels, want) {
			t.Errorf("the greeter preview shows %v, want %q", labels, want)
		}
	}
	if users := treetest.WithClass(preview, "lock-user-button"); len(users) != len(lockPreviewAccounts()) || !slices.Contains(users[0].(*widget.Button).Classes(), "selected") {
		t.Errorf("the greeter preview lists %d accounts, the first selected: want %d", len(users), len(lockPreviewAccounts()))
	}
	if shownWithClass(t, preview, "lock-user") || shownWithClass(t, preview, "lock-media") || shownWithClass(t, preview, "lock-switch-user") {
		t.Error("the greeter preview shows the username entry over its list, or the lock's own rows")
	}
	if !shownWithClass(t, preview, "lock-fingerprint") {
		t.Error("the greeter's fingerprint previews no reader")
	}
	untippedIconButtons(t, "the greeter preview", preview)

	set(t, k, p, "login.greeter.show-user-list", false)
	set(t, k, p, "login.greeter.show-power-buttons", false)
	set(t, k, p, "login.greeter.fingerprint", false)
	preview, _ = previewedLock(t, p)
	if shownWithClass(t, preview, "lock-users") || !shownWithClass(t, preview, "lock-user") {
		t.Error("show-user-list off previews the list instead of the username entry")
	}
	if len(treetest.WithClass(preview, "lock-power-button")) != 0 {
		t.Error("show-power-buttons off previews power buttons")
	}
	if shownWithClass(t, preview, "lock-fingerprint") || !shownWithClass(t, preview, "lock-secret") {
		t.Error("the greeter's fingerprint off previews the reader")
	}

	draft := runtimeFile(t, k)
	lockPreviewMode(t, p, lockview.Lock)
	preview, _ = previewedLock(t, p)
	if len(treetest.WithClass(preview, "lock-user")) != 0 {
		t.Error("switching back to the lock screen keeps the greeter's username entry")
	}
	if got := runtimeFile(t, k); got != draft {
		t.Errorf("switching the previewed screen rewrote the draft %q as %q", draft, got)
	}
}

func TestTheLockPreviewShowsIconsTheAppShips(t *testing.T) {
	listed := listedIcons(t)
	for _, icon := range slices.Concat(lockview.Icons(), lockview.MediaIcons(), lockview.PowerIcons()) {
		if !slices.Contains(listed, icon) {
			t.Errorf("the lock preview draws %s, which resources/icons.txt does not list", icon)
		}
	}
}
