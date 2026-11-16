package bar

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/clipboard"
	"github.com/stubbedev/lys/desktop/service/clipboard/historybudget"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"

	"github.com/stubbedev/lys/shell/internal/apptest"
	"github.com/stubbedev/lys/shell/internal/keytest"
	"github.com/stubbedev/lys/shell/shell/screenshot"
	"github.com/stubbedev/lys/shell/shell/shotthumb"
)

func TestEditorSaveWritesAndCopiesWithoutStallingTheLoop(t *testing.T) {
	cfg := config.Defaults()
	ctx := styledContext(t, cfg)
	a := apptest.Run(t)
	history, err := clipboard.DefaultHistory(historybudget.DefaultBudget(), historybudget.DefaultPinnedBudget())
	if err != nil {
		t.Fatal(err)
	}
	var clip *clipboard.Clipboard
	if err := apptest.On(a, func() error {
		a.SetClipboard(app.NewClipboard(a.Session()))
		var err error
		clip, err = clipboard.Start(a, history, clipboard.Stores{})
		return err
	}); err != nil {
		t.Fatalf("the shell's clipboard on sway: %v", err)
	}
	t.Cleanup(func() { apptest.On(a, func() bool { clip.Close(); return true }) })
	svc := apptest.On(a, func() *captureServices {
		return newCaptureServices(a, a.Session().Outputs, func() *config.Config { return cfg }, nil, clip, ctx.Font, ctx.Theme.Sheet, nil)
	})
	t.Cleanup(func() { apptest.On(a, func() bool { svc.close(); return true }) })

	path := filepath.Join(t.TempDir(), "shot.png")
	capture := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for i := range capture.Pix {
		capture.Pix[i] = 0xff
	}
	apptest.On(a, func() bool {
		shot := svc.shots.hold(screenshot.Saved{Path: path, Image: capture})
		svc.shots.edit(shot)
		svc.shots.release(shot)
		return true
	})
	keys := keytest.Dial(t, a)
	keytest.AwaitFocus(t, a)
	keys.Tap(keytest.Return)

	apptest.Until(t, a, "the editor closed and let the capture go", func() bool { return len(svc.shots.holders) == 0 })
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Enter in the editor wrote nothing: %v", err)
	}
	saved := decodePNG(t, data)
	if saved.Bounds() != capture.Bounds() || saved.RGBAAt(10, 10) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("saved a %v image with %v at (10,10), want the unmarked %v capture", saved.Bounds(), saved.RGBAAt(10, 10), capture.Bounds())
	}
	snap := apptest.On(a, clip.Entries)
	defer snap.Release()
	if entries := snap.Entries(); len(entries) != 1 || entries[0].Mime != transfer.MimePNG {
		t.Fatalf("the clipboard history holds %v, want the edited image as image/png", entries)
	}
}

// The screenshot surfaces follow the styling scale: at 1 the body text,
// a 12px padding and a 48px gap on a 16px root, and twice each at 2.
func TestShotSizesFollowTheScale(t *testing.T) {
	cfg := config.Defaults()
	one := shotSizesOf(cfg.Styling)
	if one.textPx != widgets.TextPx(styling.Body, 1) || one.padding != 12 || one.margin != 48 {
		t.Errorf("scale 1: %+v, want the body text, 12px padding, 48px margin", one)
	}
	cfg.Styling.Scale = 2
	two := shotSizesOf(cfg.Styling)
	if two.textPx != 2*one.textPx || two.padding != 2*one.padding || two.margin != 2*one.margin {
		t.Errorf("scale 2: %+v, want twice %+v", two, one)
	}
}

func TestDeletingAScreenshotFromItsThumbnailToastsAFailure(t *testing.T) {
	shown := &toasts{}
	u := &shotUI{toast: shown.record, saved: map[render.ImageID]screenshot.Saved{}}
	dir := t.TempDir()
	kept := filepath.Join(dir, "kept.png")
	if err := os.WriteFile(kept, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	u.discard(shotthumb.Shot{Path: kept})
	waitHeadless(t, "the screenshot deleted", func() bool {
		_, err := os.Stat(kept)
		return errors.Is(err, os.ErrNotExist)
	})
	if got := shown.list(); len(got) != 0 {
		t.Fatalf("a delete that went through toasted %q", got)
	}

	gone := filepath.Join(dir, "gone.png")
	u.discard(shotthumb.Shot{Path: gone})
	waitHeadless(t, "the failed delete toasted", func() bool { return len(shown.list()) == 1 })
	want := i18n.T("screenshot-delete-failed", i18n.Str("error", screenshot.Saved{Path: gone}.Discard().Error()))
	if got := shown.list(); got[0] != want {
		t.Errorf("toasted %q, want %q", got[0], want)
	}
}

func TestCopyingAnEditedScreenshotToastsAFailure(t *testing.T) {
	shown := &toasts{}
	var copyErr error
	copies := 0
	u := &shotUI{toast: shown.record, copy: func(transfer.Content) (func(), error) {
		copies++
		return func() {}, copyErr
	}}
	u.copyEdited(transfer.Content{}, nil)
	if got := shown.list(); copies != 1 || len(got) != 0 {
		t.Fatalf("a copy that went through: %d copies, toasts %q", copies, got)
	}
	copyErr = errors.New("no clipboard owner")
	u.copyEdited(transfer.Content{}, nil)
	u.copyEdited(transfer.Content{}, errors.New("png: invalid format"))
	want := []string{
		i18n.T("screenshot-copy-edited-failed", i18n.Str("error", "no clipboard owner")),
		i18n.T("screenshot-copy-edited-failed", i18n.Str("error", "png: invalid format")),
	}
	if got := shown.list(); copies != 2 || !slices.Equal(got, want) {
		t.Errorf("%d copies, toasts %q; want two copies, the image that did not encode not offered, and %q", copies, got, want)
	}
}
