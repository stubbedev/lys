package settings

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// stockAvatarChoice opens the picture dialog from the header's avatar
// and hands back its i-th stock picture.
func stockAvatarChoice(t *testing.T, p *usersPanel, i int) *widget.Button {
	t.Helper()
	pickers := &fakePickers{}
	p.k.pickers = pickers
	treetest.Button(t, p, locale.T().Get("settings-users-avatar-change")).OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("the avatar opened %d dialogs, want the picture dialog", len(pickers.dialogs))
	}
	var choices []*widget.Button
	for _, b := range treetest.All[*widget.Button](pickers.dialogs[0].Content) {
		if len(treetest.All[*widget.Avatar](b)) == 1 {
			choices = append(choices, b)
		}
	}
	if len(choices) <= i {
		t.Fatalf("the picture dialog offers %d stock pictures", len(choices))
	}
	b := choices[i]
	click := b.OnClick
	b.OnClick = func() {
		click()
		if pickers.dialogsClosed != 1 {
			t.Errorf("choosing a stock picture closed the dialog %d times, want once", pickers.dialogsClosed)
		}
	}
	return b
}

func halves(w, h int) *image.RGBA {
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			c := color.RGBA{0, 0, 255, 255}
			if x < w/2 {
				c = color.RGBA{255, 0, 0, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	return src
}

func TestAnAvatarCropZoomsAndPansInsideThePicture(t *testing.T) {
	src := halves(400, 200)
	left := cropAvatar(src, avatarCrop{zoom: 1, panX: -1})
	right := cropAvatar(src, avatarCrop{zoom: 1, panX: 1})
	if left.RGBAAt(avatarSide/2, avatarSide/2).R != 255 || right.RGBAAt(avatarSide/2, avatarSide/2).B != 255 {
		t.Fatalf("panning left shows %v, right %v", left.RGBAAt(avatarSide/2, avatarSide/2), right.RGBAAt(avatarSide/2, avatarSide/2))
	}
	if b := cropAvatar(src, avatarCrop{zoom: 3, panX: 5, panY: -5}).Bounds(); b.Dx() != avatarSide || b.Dy() != avatarSide {
		t.Fatalf("%v", b)
	}
}

func TestDraggingThePictureMovesTheCropTheOtherWayUpToItsEdge(t *testing.T) {
	wide := image.Rect(0, 0, 400, 200)
	c := avatarCrop{zoom: 1}
	if got := c.dragged(wide, 50, 0, 200); got.panX >= 0 || got.panY != 0 {
		t.Fatalf("dragging the picture right moved the crop to %+v, want left", got)
	}
	if got := c.dragged(wide, -10000, 0, 200); got.panX != 1 {
		t.Fatalf("dragging far left left the crop at %+v, want the right edge", got)
	}
	if got := c.dragged(wide, 0, 50, 200); got.panY != 0 {
		t.Errorf("a picture as tall as the crop panned up and down to %+v", got)
	}
	if got := c.dragged(wide, 50, 0, 0); got != c {
		t.Errorf("a crop drawn nowhere moved to %+v", got)
	}
	if got := c.zoomed(1); got.zoom <= 1 {
		t.Errorf("zooming in gave %v", got.zoom)
	}
	if got := c.zoomed(-3); got.zoom != avatarZoomMin {
		t.Errorf("zooming out past the whole picture gave %v", got.zoom)
	}
	if got := (avatarCrop{zoom: avatarZoomMax}).zoomed(5); got.zoom != avatarZoomMax {
		t.Errorf("zooming in past the limit gave %v", got.zoom)
	}
}

func TestTheCropperDrawsTheCropAndFollowsDragsAndKeys(t *testing.T) {
	var seen []avatarCrop
	c := newAvatarCropper(halves(4000, 2000), func(crop avatarCrop) { seen = append(seen, crop) })
	if b := c.work.Bounds(); min(b.Dx(), b.Dy()) != avatarWorkSide {
		t.Fatalf("the working copy is %v, want its short side %d", b, avatarWorkSide)
	}
	c.Arrange(render.Rect{W: avatarCropSide, H: avatarCropSide})
	c.PressAt(widget.Point{X: 100, Y: 100})
	c.DragMove(widget.Point{X: 160, Y: 100})
	if c.crop.panX >= 0 || len(seen) != 1 {
		t.Fatalf("dragging right left the crop at %+v and reported %v", c.crop, seen)
	}
	before := c.crop
	c.KeyAction(widget.KeyRight, 0)
	if c.crop.panX <= before.panX {
		t.Errorf("the right key moved the crop from %+v to %+v", before, c.crop)
	}
	c.DragMove(widget.Point{X: 160, Y: 100})
	c.DragMove(widget.Point{X: 160, Y: 100})
	if n := len(seen); n != 3 {
		t.Errorf("an unchanged crop was reported again: %d reports", n)
	}
}

func avatarFile(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, halves(300, 200)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "me.png")
	put(t, path, buf.String())
	return path
}

func TestAPictureFromAFileIsCroppedInADialogAndSetOnApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	k := testKit(t, "")
	pickers := &fakePickers{}
	k.pickers = pickers
	p := &usersPanel{Box: gapBox(widget.Column), k: k}
	p.problem = k.inlineState()
	p.face = widget.NewAvatar(k.face, identityAvatar, "")
	p.openAvatarDialog()
	treetest.Button(t, pickers.dialogs[0].Content, "Choose a File…").OnClick()
	if pickers.dialogsClosed != 1 || pickers.fileFn == nil {
		t.Fatalf("choosing a file closed the picture dialog %d times and asked for a file %v", pickers.dialogsClosed, pickers.fileFn != nil)
	}
	pickers.fileFn(avatarFile(t))
	if len(pickers.dialogs) != 2 {
		t.Fatalf("a picture file opened %d dialogs, want the crop dialog second", len(pickers.dialogs))
	}
	crop := pickers.dialogs[1]
	cropper, ok := crop.Focus.(*avatarCropper)
	if !ok || !crop.Modal || crop.CancelResponse != responseCancel || crop.DefaultResponse != responseConfirm {
		t.Fatalf("the crop dialog focuses %T, modal %v, cancel %q, default %q", crop.Focus, crop.Modal, crop.CancelResponse, crop.DefaultResponse)
	}
	zoom := treetest.First[*widgets.DebouncedSlider](t, crop.Content).Knob
	zoom.OnChanged(2)
	if cropper.crop.zoom != 2 {
		t.Fatalf("the zoom slider left the crop at %+v", cropper.crop)
	}
	cropper.set(cropper.crop.zoomed(1))
	if zoom.Value() != cropper.crop.zoom {
		t.Fatalf("the slider reads %v while the crop zooms %v", zoom.Value(), cropper.crop.zoom)
	}
	face := filepath.Join(home, ".face")
	pickers.respond(responseCancel)
	if _, err := os.Stat(face); !os.IsNotExist(err) {
		t.Fatalf("cancelling the crop set the picture: %v", err)
	}
	pickers.respond(responseConfirm)
	data, err := os.ReadFile(face)
	if err != nil {
		t.Fatalf("applying the crop set no picture: %v", err)
	}
	if img, err := png.Decode(bytes.NewReader(data)); err != nil || img.Bounds().Dx() != avatarSide {
		t.Fatalf("the picture set is %v, %v", img, err)
	}
}

func TestAFileThatIsNoPictureSaysSoAndOpensNoCrop(t *testing.T) {
	k := testKit(t, "")
	pickers := &fakePickers{}
	k.pickers = pickers
	p := &usersPanel{Box: gapBox(widget.Column), k: k}
	p.problem = k.inlineState()
	broken := filepath.Join(t.TempDir(), "notes.png")
	put(t, broken, "not a picture")
	p.cropFile(broken)
	if len(pickers.dialogs) != 0 || !p.problem.Visible() || !strings.Contains(p.problem.Text(), broken) {
		t.Fatalf("a broken file opened %d dialogs and says %q", len(pickers.dialogs), p.problem.Text())
	}
}

func TestWithoutAWindowThePictureCannotBeChanged(t *testing.T) {
	k := testKit(t, "")
	p := &usersPanel{Box: gapBox(widget.Column), k: k}
	p.problem = k.inlineState()
	p.openAvatarDialog()
	if !p.problem.Visible() || !strings.Contains(p.problem.detail.Text(), "no window") {
		t.Fatalf("without a window the page says %q over %q", p.problem.Text(), p.problem.detail.Text())
	}
}
