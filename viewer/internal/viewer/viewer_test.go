package viewer

import (
	"bytes"
	"errors"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/viewer/internal/locale"
)

func says(v *Viewer, message string) bool {
	return slices.Contains(treetest.Labels(v.Root()), message)
}

func TestAPictureFillsTheWindow(t *testing.T) {
	h := newHarness(t)
	pic := writeFile(t, filepath.Join(t.TempDir(), "pano.png"), pngData(t, 80, 20))
	v := h.viewer()
	h.open(v, pic)
	layOut(v)
	if v.View().Renderer() != preview.Image {
		t.Fatalf("pano.png shows as %v", v.View().Renderer())
	}
	if b := v.View().Picture().Bounds(); b.W < 1000 || b.H < 600 {
		t.Errorf("the picture shows in %+v, want most of the 1200x900 window", b)
	}
	if h.titles[len(h.titles)-1] != "pano.png" {
		t.Errorf("the window is titled %q", h.titles[len(h.titles)-1])
	}
	if !v.copy.Enabled() {
		t.Error("a picture cannot be copied")
	}
}

func TestAnAnimatedPictureShowsAsAPicture(t *testing.T) {
	h := newHarness(t)
	anim := writeFile(t, filepath.Join(t.TempDir(), "spin.gif"), animatedGIF(t, 3))
	v := h.viewer()
	h.open(v, anim)
	layOut(v)
	if v.View().Renderer() != preview.Image || v.View().Picture().Bounds().Empty() {
		t.Fatalf("spin.gif shows as %v in %+v", v.View().Renderer(), v.View().Picture().Bounds())
	}
}

func h264Clip(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "ui", "media", "testdata", "h264-aac.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	return writeFile(t, filepath.Join(t.TempDir(), "clip.mp4"), data)
}

func TestAnH264MP4PlaysInTheVideoWithItsControls(t *testing.T) {
	h := newHarness(t)
	clip := h264Clip(t)
	asked := 0
	remove := widget.SetTextureHost(func(*widget.Video, int, int, func(), func()) (widget.TextureSurface, error) {
		asked++
		return nil, widget.ErrVideoDevicePending
	})
	t.Cleanup(remove)
	v := h.viewer()
	h.open(v, clip)
	layOut(v)
	videos := treetest.All[*widget.Video](v.Root())
	if v.View().Renderer() != preview.Video || len(videos) != 1 || videos[0].Stream() == nil || videos[0].Stream() != v.View().Stream() {
		t.Fatalf("clip.mp4 shows as %v with %d videos", v.View().Renderer(), len(videos))
	}
	if videos[0].Controls() == nil || !v.loop.Visible() || v.copy.Enabled() {
		t.Errorf("the video's controls %v, loop shown %v, copy enabled %v", videos[0].Controls(), v.loop.Visible(), v.copy.Enabled())
	}
	if b := videos[0].Bounds(); b.W < 1000 || b.H < 600 {
		t.Errorf("the video shows in %+v, want most of the window", b)
	}
	if asked == 0 || v.View().Stream().Err() != nil {
		t.Errorf("the video asked for a GPU surface %d times, its stream failed with %v", asked, v.View().Stream().Err())
	}
}

func TestAVideoNothingCanDecodeSaysSo(t *testing.T) {
	h := newHarness(t)
	v := h.viewer()
	h.open(v, h264Clip(t))
	layOut(v)
	h.until("the failed video", func() bool { return v.View().Renderer() == preview.Failed })
	if !says(v, i18n.T("preview-failed-unsupported")) || v.View().Stream() != nil || v.loop.Visible() {
		t.Errorf("a video with no GPU to decode it says %q, stream %v, loop shown %v", treetest.Labels(v.Root()), v.View().Stream(), v.loop.Visible())
	}
}

func TestAFileThatCannotBeShownSaysWhy(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		path, message string
	}{
		{writeFile(t, filepath.Join(dir, "clip.mp4"), []byte("not a movie at all")), "preview-failed-unsupported"},
		{writeFile(t, filepath.Join(dir, "broken.png"), []byte("\x89PNG\r\n\x1a\n")), "preview-failed-undecodable"},
		{filepath.Join(dir, "gone.png"), "preview-failed-unreadable"},
	} {
		h := newHarness(t)
		v := h.viewer()
		v.Open(Files(c.path))
		h.landed(v)
		if v.View().Renderer() != preview.Failed || !says(v, i18n.T(c.message)) {
			t.Errorf("%s shows as %v saying %q, want %q", filepath.Base(c.path), v.View().Renderer(), treetest.Labels(v.Root()), i18n.T(c.message))
		}
		if v.copy.Enabled() || v.loop.Visible() {
			t.Errorf("%s can be copied (%v) or looped (%v)", filepath.Base(c.path), v.copy.Enabled(), v.loop.Visible())
		}
	}
}

func TestTheArrowsWalkTheFolderAsTheFileChooserSortsIt(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	for _, name := range []string{"b.png", "A.jpg", "10.png", "2.png", "d.mp4", "notes.txt", "paper.pdf", ".hidden.png", "scan.ppm"} {
		writeFile(t, filepath.Join(dir, name), pngData(t, 4, 4))
	}
	if err := os.Mkdir(filepath.Join(dir, "sub.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	v := h.viewer()
	h.open(v, filepath.Join(dir, "b.png"))
	var names []string
	for _, p := range v.Files().Paths() {
		names = append(names, filepath.Base(p))
	}
	if want := []string{"10.png", "2.png", "A.jpg", "b.png", "d.mp4", "paper.pdf", "scan.ppm"}; !slices.Equal(names, want) {
		t.Fatalf("the folder walks %v, want %v", names, want)
	}
	if current(t, v) != "b.png" || !v.previous.Enabled() || !v.next.Enabled() {
		t.Fatalf("opened on %s, previous %v next %v", current(t, v), v.previous.Enabled(), v.next.Enabled())
	}
	for _, step := range []struct{ keys, want string }{
		{"Right", "d.mp4"},
		{"Right", "paper.pdf"},
		{"Right", "scan.ppm"},
		{"Right", "scan.ppm"},
		{"Left", "paper.pdf"},
		{"Left", "d.mp4"},
		{"Home", "10.png"},
		{"Left", "10.png"},
		{"End", "scan.ppm"},
		{"Prior", "paper.pdf"},
		{"Next", "scan.ppm"},
	} {
		if !h.press(v, step.keys) || current(t, v) != step.want {
			t.Fatalf("%s showed %s, want %s", step.keys, current(t, v), step.want)
		}
	}
	if v.next.Enabled() || !v.previous.Enabled() {
		t.Errorf("at the last file next is %v and previous %v", v.next.Enabled(), v.previous.Enabled())
	}
	h.open(v, filepath.Join(dir, ".hidden.png"))
	if current(t, v) != ".hidden.png" || len(v.Files().Paths()) != 8 {
		t.Errorf("a hidden file opened walks %d files from %s, want 8 from itself", len(v.Files().Paths()), current(t, v))
	}
	h.open(v, dir)
	if current(t, v) != "10.png" || len(v.Files().Paths()) != 7 {
		t.Errorf("the folder opened walks %d files from %s, want 7 from 10.png", len(v.Files().Paths()), current(t, v))
	}
	h.open(v, filepath.Join(dir, "b.png"), filepath.Join(dir, "notes.txt"))
	if got := v.Files().Paths(); len(got) != 2 || filepath.Base(got[1]) != "notes.txt" {
		t.Errorf("two files opened walk %v, want just them", got)
	}
}

func inTrash(t *testing.T, h *harness) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(h.bin.DataHome, "Trash", "files"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestTrashingAFileShowsTheNextAndUndoBringsItBack(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		writeFile(t, filepath.Join(dir, name), pngData(t, 4, 4))
	}
	v := h.viewer()
	h.open(v, filepath.Join(dir, "b.png"))
	if h.press(v, "ctrl+z") {
		t.Error("undo took the key with nothing trashed")
	}
	h.press(v, "Delete")
	h.until("b.png trashed", func() bool { return len(h.toasts) == 1 })
	h.landed(v)
	if current(t, v) != "c.png" || len(v.Files().Paths()) != 2 {
		t.Fatalf("after trashing b.png the viewer shows %s of %d", current(t, v), len(v.Files().Paths()))
	}
	if _, err := os.Stat(filepath.Join(dir, "b.png")); !errors.Is(err, os.ErrNotExist) || !slices.Equal(inTrash(t, h), []string{"b.png"}) {
		t.Fatalf("b.png is still there (%v), the trash holds %v", err, inTrash(t, h))
	}
	if want := locale.T().Get("viewer-trashed", i18n.Str("name", "b.png")); h.toasts[0].text != want || h.toasts[0].undo == nil {
		t.Fatalf("the toast says %q with undo %v, want %q", h.toasts[0].text, h.toasts[0].undo != nil, want)
	}
	h.press(v, "Delete")
	h.until("c.png trashed", func() bool { return len(h.toasts) == 2 })
	h.landed(v)
	if current(t, v) != "a.png" {
		t.Fatalf("after trashing the last file the viewer shows %s, want the one before", current(t, v))
	}
	h.toasts[1].undo()
	h.until("c.png restored", func() bool { return len(v.Files().Paths()) == 2 })
	h.landed(v)
	if current(t, v) != "c.png" {
		t.Fatalf("after undoing the viewer shows %s, want c.png", current(t, v))
	}
	if !h.press(v, "ctrl+z") {
		t.Fatal("undo left the key with b.png trashed")
	}
	h.until("b.png restored", func() bool { return len(v.Files().Paths()) == 3 })
	h.landed(v)
	if got := v.Files().Paths(); current(t, v) != "b.png" || filepath.Base(got[1]) != "b.png" {
		t.Fatalf("after undoing again the viewer shows %s of %v", current(t, v), got)
	}
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s is not back: %v", name, err)
		}
	}
	if left := inTrash(t, h); len(left) != 0 {
		t.Errorf("the trash still holds %v", left)
	}
	if h.press(v, "ctrl+z") {
		t.Error("undo took the key with everything restored")
	}
}

func TestAFileThatCannotBeTrashedStaysAndSaysSo(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	pic := writeFile(t, filepath.Join(dir, "a.png"), pngData(t, 4, 4))
	writeFile(t, filepath.Join(dir, "b.png"), pngData(t, 4, 4))
	v := h.viewer()
	h.open(v, pic)
	if err := os.Remove(pic); err != nil {
		t.Fatal(err)
	}
	h.press(v, "Delete")
	h.until("the failed trash", func() bool { return len(h.toasts) == 1 })
	if want := locale.T().Get("viewer-trash-failed", i18n.Str("name", "a.png")); h.toasts[0].text != want || h.toasts[0].undo != nil {
		t.Errorf("the toast says %q with undo %v, want %q", h.toasts[0].text, h.toasts[0].undo != nil, want)
	}
	if current(t, v) != "a.png" || len(v.Files().Paths()) != 2 || h.press(v, "ctrl+z") {
		t.Errorf("the failed trash left %s of %v, with something to undo", current(t, v), v.Files().Paths())
	}
}

func TestTrashingTheOnlyFileLeavesNothingShown(t *testing.T) {
	h := newHarness(t)
	v := h.viewer()
	h.open(v, writeFile(t, filepath.Join(t.TempDir(), "only.png"), pngData(t, 4, 4)))
	h.press(v, "Delete")
	h.until("only.png trashed", func() bool { return len(h.toasts) == 1 })
	if _, ok := v.Files().Current(); ok || v.View().Renderer() != preview.Empty || v.trash.Enabled() || v.openWith.Enabled() {
		t.Fatalf("the viewer shows %v with trash %v", v.View().Renderer(), v.trash.Enabled())
	}
	if want := locale.T().Get("viewer-empty"); h.titles[len(h.titles)-1] != want || !says(v, want) {
		t.Errorf("the empty viewer is titled %q", h.titles[len(h.titles)-1])
	}
	if !h.press(v, "Right") || !h.press(v, "Delete") {
		t.Error("the empty viewer left its keys")
	}
}

func TestCopyingAPictureOffersItTurnedAsItShows(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	pic := writeFile(t, filepath.Join(dir, "a.png"), pngData(t, 6, 3))
	clip := writeFile(t, filepath.Join(dir, "b.y4m"), y4mClip(16, 16, 2))
	v := h.viewer()
	h.open(v, pic)
	h.press(v, "ctrl+c")
	h.until("the copy", func() bool { return len(h.toasts) == 1 })
	if len(h.copied) != 1 || !slices.Contains(h.copied[0].Mimes, transfer.MimePNG) || h.toasts[0].text != locale.T().Get("viewer-copied") {
		t.Fatalf("copied %v, toasts %v", h.copied, h.toasts)
	}
	var png bytes.Buffer
	if err := h.copied[0].Write(transfer.MimePNG, &png); err != nil || !bytes.HasPrefix(png.Bytes(), []byte("\x89PNG")) {
		t.Errorf("the copied PNG is %d bytes: %v", png.Len(), err)
	}
	h.copyErr = errors.New("no clipboard")
	h.press(v, "ctrl+c")
	h.until("the failed copy", func() bool { return len(h.toasts) == 2 })
	if h.toasts[1].text != locale.T().Get("viewer-copy-failed") || len(h.copied) != 1 {
		t.Errorf("a refused copy says %q", h.toasts[1].text)
	}
	h.copyErr = nil
	h.open(v, clip)
	h.press(v, "ctrl+c")
	h.work.Wait()
	h.drain()
	if len(h.copied) != 1 || len(h.toasts) != 2 {
		t.Errorf("copying a video copied %d and said %v", len(h.copied)-1, h.toasts[2:])
	}
}

func TestTurningAPictureForTheClipboard(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 6, 3))
	img.Pix[0] = 255
	turned, err := asShown(img, exif.RightTop)
	if err != nil {
		t.Fatal(err)
	}
	if b := turned.Bounds(); b.Dx() != 3 || b.Dy() != 6 {
		t.Fatalf("turned a quarter the 6x3 picture is %v", b)
	}
	if r, _, _, _ := turned.At(2, 0).RGBA(); r == 0 {
		t.Error("turned a quarter clockwise, the top left pixel is not at the top right")
	}
	if same, err := asShown(img, exif.TopLeft); err != nil || same != image.Image(img) {
		t.Errorf("a picture stored as shown was redrawn: %v", err)
	}
}

func TestTheKeysPlayAVideoAndStepPastAPicture(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	clip := writeFile(t, filepath.Join(dir, "a.y4m"), y4mClip(64, 32, 4))
	writeFile(t, filepath.Join(dir, "b.png"), pngData(t, 4, 4))
	writeFile(t, filepath.Join(dir, "c.png"), pngData(t, 4, 4))
	v := h.viewer()
	h.open(v, clip)
	s := v.View().Stream()
	if s == nil || !s.Playing() {
		t.Fatalf("a.y4m shows as %v, playing %v", v.View().Renderer(), s != nil && s.Playing())
	}
	h.press(v, "space")
	if s.Playing() || current(t, v) != "a.y4m" {
		t.Error("space did not pause the video")
	}
	h.press(v, "space")
	if !s.Playing() {
		t.Error("space did not play the video again")
	}
	h.press(v, "l")
	if !s.Loop() || !v.loop.Active() {
		t.Error("l did not loop the video")
	}
	h.press(v, "Down")
	if s.Volume() >= 1 {
		t.Errorf("Down left the volume at %v", s.Volume())
	}
	h.press(v, "Up")
	h.press(v, "m")
	if s.Volume() != 1 || !s.Muted() {
		t.Errorf("Up and m left the volume at %v, muted %v", s.Volume(), s.Muted())
	}
	if !h.press(v, "shift+Right") || current(t, v) != "a.y4m" {
		t.Error("shift+Right left the video")
	}
	h.press(v, "Right")
	if current(t, v) != "b.png" || !s.Closed() {
		t.Fatalf("Right showed %s, the video closed %v", current(t, v), s.Closed())
	}
	if h.press(v, "Up") || h.press(v, "l") || h.press(v, "m") {
		t.Error("a picture took a video's key")
	}
	h.press(v, "space")
	if current(t, v) != "c.png" {
		t.Errorf("space on a picture showed %s, want the next", current(t, v))
	}
	h.press(v, "Home")
	if v.View().Stream() == nil || !v.loop.Active() || !v.View().Stream().Loop() {
		t.Error("a video shown again does not loop as the toggle says")
	}
}

func TestZoomKeysPassToThePictureAndTheArrowsPanItZoomed(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.png"), pngData(t, 1600, 1200))
	writeFile(t, filepath.Join(dir, "b.png"), pngData(t, 4, 4))
	v := h.viewer()
	h.open(v, filepath.Join(dir, "a.png"))
	layOut(v)
	p := v.View().Picture()
	if !p.Fitted() {
		t.Fatal("the picture opens zoomed")
	}
	for _, keys := range []string{"plus", "shift+plus", "equal", "KP_Add"} {
		before := p.Zoom()
		if !h.press(v, keys) || p.Zoom() <= before || p.Fitted() {
			t.Errorf("%s zoomed from %v to %v", keys, before, p.Zoom())
		}
	}
	if !h.press(v, "Right") || current(t, v) != "a.png" {
		t.Errorf("Right on a zoomed picture showed %s, want it panned", current(t, v))
	}
	before := p.Zoom()
	if !h.press(v, "minus") || p.Zoom() >= before {
		t.Errorf("minus zoomed from %v to %v", before, p.Zoom())
	}
	h.press(v, "1")
	if p.Zoom() != 1 {
		t.Errorf("1 zoomed to %v, want actual size", p.Zoom())
	}
	h.press(v, "0")
	if !p.Fitted() {
		t.Error("0 did not fit the picture")
	}
	h.press(v, "Right")
	if current(t, v) != "b.png" {
		t.Errorf("Right on a fitted picture showed %s, want the next", current(t, v))
	}
	h.open(v, writeFile(t, filepath.Join(dir, "c.y4m"), y4mClip(16, 16, 2)))
	if h.press(v, "plus") || h.press(v, "0") {
		t.Error("a video took a zoom key")
	}
}

func TestF11TogglesFullscreenAndEscapeLeavesIt(t *testing.T) {
	h := newHarness(t)
	v := h.viewer()
	h.open(v, writeFile(t, filepath.Join(t.TempDir(), "a.png"), pngData(t, 4, 4)))
	if h.press(v, "Escape") {
		t.Error("Escape outside fullscreen was taken")
	}
	h.press(v, "F11")
	if !slices.Equal(h.fullscreen, []bool{true}) || v.bar.Visible() {
		t.Fatalf("F11 asked %v with the toolbar shown %v", h.fullscreen, v.bar.Visible())
	}
	if !h.press(v, "Escape") || !slices.Equal(h.fullscreen, []bool{true, false}) || !v.bar.Visible() {
		t.Fatalf("Escape asked %v with the toolbar shown %v", h.fullscreen, v.bar.Visible())
	}
	h.press(v, "F11")
	h.press(v, "F11")
	if !slices.Equal(h.fullscreen, []bool{true, false, true, false}) {
		t.Errorf("F11 twice asked %v", h.fullscreen)
	}
}

func TestOpenWithAsksTheHostForTheFileShown(t *testing.T) {
	h := newHarness(t)
	pic := writeFile(t, filepath.Join(t.TempDir(), "a.png"), pngData(t, 4, 4))
	v := h.viewer()
	h.open(v, pic)
	treetest.Button(t, v.Root(), locale.T().Get("viewer-open-with")).OnClick()
	if !slices.Equal(h.openedWith, []string{pic}) {
		t.Errorf("open with asked for %v, want %s", h.openedWith, pic)
	}
}

func TestOpenWithListsEveryOtherAppForTheType(t *testing.T) {
	dir := t.TempDir()
	apps := filepath.Join(dir, "applications")
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lys-viewer", "paint", "player"} {
		fdtest.WriteExecutable(t, filepath.Join(bin, name), []byte("#!/bin/sh\n"))
	}
	t.Setenv("PATH", bin)
	own, err := os.ReadFile(filepath.Join("..", "..", "resources", AppID+".desktop"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(apps, AppID+".desktop"), own)
	writeFile(t, filepath.Join(apps, "paint.desktop"), []byte("[Desktop Entry]\nType=Application\nName=Paint\nExec=paint %f\nMimeType=image/png;\n"))
	writeFile(t, filepath.Join(apps, "player.desktop"), []byte("[Desktop Entry]\nType=Application\nName=Player\nExec=player %f\nMimeType=video/mp4;\n"))
	ids := func(mimeType string) []string {
		var out []string
		for _, a := range otherApps([]string{apps}, nil, mimeType) {
			out = append(out, a.ID)
		}
		return out
	}
	if got := ids("image/png"); !slices.Equal(got, []string{"paint.desktop"}) {
		t.Errorf("a PNG opens with %v, want paint alone", got)
	}
	if got := ids("video/mp4"); !slices.Equal(got, []string{"player.desktop"}) {
		t.Errorf("an MP4 opens with %v, want player alone", got)
	}
	if got := ids("application/pdf"); len(got) != 0 {
		t.Errorf("a PDF opens with %v", got)
	}
	if _, ok := desktopentry.FindApp([]string{apps}, AppID+".desktop"); !ok {
		t.Error("the viewer's own entry does not load, so leaving it out proves nothing")
	}
}

func TestPathsAreAbsoluteLocalFiles(t *testing.T) {
	got, err := Paths([]string{"a.png", "./b/../c.png", "/srv/d.png", "file:///srv/with%20space.png", "file://localhost/srv/e.png"}, "/home/me")
	want := []string{"/home/me/a.png", "/home/me/c.png", "/srv/d.png", "/srv/with space.png", "/srv/e.png"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Paths = %v, %v; want %v", got, err, want)
	}
	for _, bad := range []string{"https://example.org/a.png", "file://host/srv/a.png", "sftp://host/a.png", ""} {
		if _, err := Paths([]string{"a.png", bad}, "/home/me"); err == nil {
			t.Errorf("%q passed as a local file", bad)
		} else if bad != "" && !strings.Contains(err.Error(), bad) {
			t.Errorf("the error for %q does not name it: %v", bad, err)
		}
	}
}
