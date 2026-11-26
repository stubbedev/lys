package preview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/imagedecode/imagedecodetest"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

func lay(root widget.Widget) {
	sz := widget.SizeWithin(root, widget.Size{W: 1200, H: 900})
	root.Arrange(render.Rect{W: sz.W, H: sz.H})
}

// Each type is shown first as its renderer: a picture imagedecode reads
// as one, a video or an audio file as a video, text as text, a folder as
// a folder, and anything else, a picture imagedecode does not read
// included, as its thumbnail.
func TestTheRendererFollowsTheMIMEType(t *testing.T) {
	db := testDB(t)
	for mimeType, want := range map[string]Renderer{
		"image/png": Image, "image/jpeg": Image, "image/gif": Image, "image/webp": Image,
		"video/mp4": Video, "video/x-y4m": Video, "audio/x-matroska": Video,
		"text/plain": Text, "text/x-go": Text,
		mime.Directory:    Folder,
		"application/pdf": Thumbnail, "image/svg+xml": Thumbnail, mime.Unknown: Thumbnail, "application/x-thing": Thumbnail,
	} {
		if got := RendererFor(db, mimeType); got != want {
			t.Errorf("RendererFor(%s) = %v, want %v", mimeType, got, want)
		}
	}
}

// A picture shows in the zoomable view at its aspect ratio, its pixel
// size in the detail line; a picture whose EXIF block turns it shows
// turned, its size as shown.
func TestAPictureShowsWholeAndTurnedAsItsFileRecords(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	wide := writeFile(t, filepath.Join(dir, "pano.png"), pngData(t, gradient(800, 200)))
	turned := writeFile(t, filepath.Join(dir, "phone.jpg"), imagedecodetest.OrientedJPEG(t, gradient(60, 20), 6))
	v := h.view(nil)
	h.show(v, wide)
	lay(v.Root())
	views := treetest.All[*widget.ImageView](v.Root())
	if v.Renderer() != Image || len(views) != 1 {
		t.Fatalf("pano.png shows as %v with %d image views", v.Renderer(), len(views))
	}
	if b := views[0].Bounds(); b.W != 4*b.H || b.W != 320 {
		t.Errorf("the 800x200 picture shows at %dx%d, want 320x80", b.W, b.H)
	}
	if info := v.info.Text(); !strings.HasPrefix(info, "PNG image · 800 × 200 · ") {
		t.Errorf("the detail line is %q", info)
	}
	h.show(v, turned)
	lay(v.Root())
	if o := views[0].Orientation(); o != exif.RightTop {
		t.Errorf("phone.jpg shows in orientation %d, want its file's 6", o)
	}
	if b := views[0].Bounds(); 3*b.W != b.H {
		t.Errorf("the 60x20 picture turned a quarter shows at %dx%d, want 1:3", b.W, b.H)
	}
	if info := v.info.Text(); !strings.Contains(info, " · 20 × 60 · ") {
		t.Errorf("the turned picture's detail line is %q, want its size as shown", info)
	}
}

// Text shows highlighted in the code view, its first TextBytes at most,
// cut on a character.
func TestTextShowsItsFirstBytesCutOnACharacter(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	src := writeFile(t, filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"))
	big := writeFile(t, filepath.Join(dir, "big.txt"), append(bytes.Repeat([]byte("a"), TextBytes-1), strings.Repeat("é", 100)...))
	v := h.view(nil)
	h.show(v, src)
	lay(v.Root())
	code := treetest.All[*widgets.CodeView](v.Root())
	if v.Renderer() != Text || len(code) != 1 || code[0].Text() != "package main\n\nfunc main() {}\n" || code[0].Name() != "main.go" {
		t.Fatalf("main.go shows as %v with %d code views", v.Renderer(), len(code))
	}
	if b := code[0].Bounds(); b.W != 320 || b.H != 200 {
		t.Errorf("the code view is %dx%d, want the 320x200 text box", b.W, b.H)
	}
	h.show(v, big)
	if text := v.code.Text(); len(text) > TextBytes || len(text) < TextBytes-2 || !utf8.ValidString(text) {
		t.Errorf("a %d byte file shows %d bytes, valid UTF-8 %v; want at most %d, cut on a character", TextBytes+199, len(text), utf8.ValidString(text), TextBytes)
	}
}

// A folder counts its items and the bytes under it; past the cap both
// are floors, and a walk stops once its context is done.
func TestAFolderCountsItsItemsAndSize(t *testing.T) {
	h := newHarness(t)
	dir := filepath.Join(t.TempDir(), "photos")
	for i := range 3 {
		writeFile(t, filepath.Join(dir, "p"+strconv.Itoa(i)+".png"), make([]byte, 1000))
	}
	writeFile(t, filepath.Join(dir, "nested", "deep.txt"), make([]byte, 2048))
	v := h.view(nil)
	h.show(v, dir)
	lay(v.Root())
	if v.Renderer() != Folder {
		t.Fatalf("a folder shows as %v", v.Renderer())
	}
	want := i18n.T("preview-kind-folder") + " · " + i18n.T("preview-folder-items", i18n.Int("count", 4), i18n.Str("shown", "4")) + " · " + byteSize(5048) + " · "
	if info := v.info.Text(); !strings.HasPrefix(info, want) {
		t.Errorf("the detail line is %q, want it to start %q", info, want)
	}

	st, err := walkFolder(context.Background(), dir, 2)
	if err != nil || st.items != 2 || !st.itemsMore || !st.bytesMore {
		t.Errorf("walking to 2 of 5 entries = %+v, %v; want 2 items, both floors", st, err)
	}
	if got := folderItems(st); got != i18n.T("preview-folder-items-capped", i18n.Str("shown", "2")) {
		t.Errorf("a capped count reads %q", got)
	}
	if got := folderBytes(st); got != i18n.T("preview-size-at-least", i18n.Str("size", byteSize(st.bytes))) {
		t.Errorf("a capped size reads %q", got)
	}
	st, err = walkFolder(context.Background(), dir, 4)
	if err != nil || st.items != 4 || st.itemsMore || !st.bytesMore {
		t.Errorf("walking to the 4 top items = %+v, %v; want the items exact, the size a floor", st, err)
	}
	ctx := &cancelAfter{Context: context.Background(), checks: 2}
	st, err = walkFolder(ctx, dir, folderWalkEntries)
	if !errors.Is(err, context.Canceled) || st.items != 2 {
		t.Errorf("a walk cancelled after 2 entries = %+v, %v; want it stopped there", st, err)
	}
}

// cancelAfter is a context cancelled once its Err was asked checks
// times.
type cancelAfter struct {
	context.Context
	checks int
}

func (c *cancelAfter) Err() error {
	if c.checks == 0 {
		return context.Canceled
	}
	c.checks--
	return nil
}

// Closing a preview while its folder is still being counted cancels the
// count, and what it found lands nowhere.
func TestClosingCancelsAFolderCount(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), []byte("a"))
	v := h.view(nil)
	h.holdLoads()
	v.Show(dir)
	h.until("the count to start", func() bool { return h.ctxOf(dir) != nil })
	v.Close()
	if err := h.ctxOf(dir).Err(); err == nil {
		t.Fatal("closing left the count running")
	}
	h.releaseLoads()
	h.work.Wait()
	h.drain()
	if v.Renderer() != Empty || v.Path() != "" || len(treetest.All[*widget.Icon](v.visual)) != 0 {
		t.Errorf("the cancelled count landed: %v %q", v.Renderer(), v.Path())
	}
}

// A video plays in the video widget with its controls, and closing the
// preview, or showing another file, closes its stream, whose decoder
// then ends.
func TestAVideoStopsDecodingWhenThePreviewCloses(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	clip := writeFile(t, filepath.Join(dir, "clip.y4m"), y4mClip(640, 320, 2))
	other := writeFile(t, filepath.Join(dir, "note.txt"), []byte("note"))
	v := h.view(func(c *Config) { c.Autoplay = true })
	h.show(v, clip)
	lay(v.Root())
	videos := treetest.All[*widget.Video](v.Root())
	if v.Renderer() != Video || len(videos) != 1 || videos[0].Stream() == nil {
		t.Fatalf("clip.y4m shows as %v with %d videos", v.Renderer(), len(videos))
	}
	stream := videos[0].Stream()
	if !stream.Playing() {
		t.Error("an autoplaying preview's video is paused")
	}
	if b := videos[0].Bounds(); b.W != 320 || b.H != 160 {
		t.Errorf("the 640x320 video shows at %dx%d, want it fitted to 320x160", b.W, b.H)
	}
	if videos[0].Controls() == nil {
		t.Error("the video has no controls")
	}
	v.Close()
	if !stream.Closed() || videos[0].Stream() != nil {
		t.Fatal("closing the preview left the video's stream open")
	}
	if err := stream.Wait(); err != nil {
		t.Errorf("the closed stream's decoder: %v", err)
	}
	h.show(v, clip)
	again := videos[0].Stream()
	h.show(v, other)
	if !again.Closed() {
		t.Error("showing another file left the video decoding")
	}
	if err := again.Wait(); err != nil {
		t.Errorf("the replaced stream's decoder: %v", err)
	}
}

// A document shows the thumbnail lys-docthumb makes, fitted to the
// box; a type nothing thumbnails, text that is not text, and a host
// without a thumbnailer show the type's icon, never a blank.
func TestAFileFallsBackToItsThumbnailThenItsIcon(t *testing.T) {
	h := newHarness(t)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	h.src.Thumbnails = thumbnail.New(thumbnail.Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Page)})
	dir := t.TempDir()
	doc := writeFile(t, filepath.Join(dir, "paper.pdf"), []byte("%PDF-1.4"))
	thing := writeFile(t, filepath.Join(dir, "blob.thing"), []byte{1, 2, 3})
	binary := writeFile(t, filepath.Join(dir, "blob.txt"), []byte("ELF\x00\x01\x02binary"))
	v := h.view(nil)
	h.show(v, doc)
	lay(v.Root())
	views := treetest.All[*widget.ImageView](v.visual)
	if v.Renderer() != Thumbnail || len(views) != 1 {
		t.Fatalf("paper.pdf shows as %v with %d image views", v.Renderer(), len(views))
	}
	if b := views[0].Bounds(); 3*b.W != 2*b.H || b.H != 240 {
		t.Errorf("the 200x300 page shows at %dx%d, want 160x240", b.W, b.H)
	}
	for _, path := range []string{thing, binary} {
		h.show(v, path)
		if v.Renderer() != Icon || len(treetest.All[*widget.Icon](v.visual)) != 1 || len(treetest.All[*widget.ImageView](v.visual)) != 0 {
			t.Errorf("%s shows as %v, want its icon alone", filepath.Base(path), v.Renderer())
		}
		if info := v.info.Text(); !strings.Contains(info, " B · ") {
			t.Errorf("%s's detail line is %q, want its size", filepath.Base(path), info)
		}
	}
	h.src.Thumbnails = nil
	v.Close()
	h.show(v, doc)
	if v.Renderer() != Icon {
		t.Errorf("without a thumbnailer paper.pdf shows as %v, want its icon", v.Renderer())
	}
}

// A file that cannot be shown says why under its icon: one that is not
// there, a picture that does not decode, one past the budget, a video
// in a format lys does not play.
func TestAFileThatCannotBeShownSaysWhy(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	cases := []struct {
		path string
		want Failure
	}{
		{filepath.Join(dir, "gone.png"), Unreadable},
		{writeFile(t, filepath.Join(dir, "bad.png"), []byte("\x89PNG\r\n\x1a\n")), Undecodable},
		{writeFile(t, filepath.Join(dir, "huge.png"), pngData(t, image.NewGray(image.Rect(0, 0, 400, 300)))), TooLarge},
		{writeFile(t, filepath.Join(dir, "clip.mp4"), []byte("not a movie at all")), Unsupported},
	}
	h.src.Pictures = NewPictures(100_000)
	v := h.view(nil)
	for _, c := range cases {
		h.show(v, c.path)
		lay(v.Root())
		if v.Renderer() != Failed || v.message.Text() != message(c.want) || v.message.Text() == "" {
			t.Errorf("%s shows as %v saying %q, want %q", filepath.Base(c.path), v.Renderer(), v.message.Text(), message(c.want))
		}
		if icons := treetest.All[*widget.Icon](v.visual); len(icons) != 1 {
			t.Errorf("%s says why under %d icons", filepath.Base(c.path), len(icons))
		}
	}
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("the failed pictures hold %d bytes of the budget", held)
	}
}

// Showing another file 50 times before the loop rests starts one load,
// of the last; a load another file superseded is cancelled.
func TestOneLoadPerRest(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	paths := make([]string, 10)
	for i := range paths {
		paths[i] = writeFile(t, filepath.Join(dir, "note"+strconv.Itoa(i)+".txt"), []byte("note "+strconv.Itoa(i)))
	}
	v := h.view(nil)
	for i := range 50 {
		v.Show(paths[i%10])
	}
	h.until("the last note", func() bool { return v.Renderer() == Text })
	if n := h.started(); n != 1 {
		t.Errorf("50 shows in one rest started %d loads, want 1", n)
	}
	if v.code.Text() != "note 9" {
		t.Errorf("the view shows %q, want the last file's text", v.code.Text())
	}
	h.holdLoads()
	v.Show(paths[3])
	h.until("the held load", func() bool { return h.ctxOf(paths[3]) != nil })
	v.Show(paths[4])
	if h.ctxOf(paths[3]).Err() == nil {
		t.Error("the superseded load was not cancelled")
	}
	h.releaseLoads()
	h.until("the next note", func() bool { return v.Renderer() == Text })
	if v.code.Text() != "note 4" {
		t.Errorf("the view shows %q, want the file shown last", v.code.Text())
	}
}

// A picture holds its bytes of the budget while it shows, and lets them
// go once another file shows or the view closes.
func TestAPictureLetsItsPixelsGo(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	pic := writeFile(t, filepath.Join(dir, "a.png"), pngData(t, gradient(64, 32)))
	note := writeFile(t, filepath.Join(dir, "b.txt"), []byte("b"))
	v := h.view(nil)
	h.show(v, pic)
	if h.src.Pictures.Held() == 0 {
		t.Fatal("a picture shows without holding its bytes")
	}
	h.show(v, note)
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("after another file showed the picture holds %d bytes", held)
	}
	h.show(v, pic)
	v.Close()
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("after closing the picture holds %d bytes", held)
	}
}

// Space and Escape close the preview through the host's callback; the
// arrows are the host's, which drives its own list; a view showing
// nothing, or a host without the callback, leaves the key alone.
func TestTheKeysCloseAndLeaveTheArrowsToTheHost(t *testing.T) {
	h := newHarness(t)
	note := writeFile(t, filepath.Join(t.TempDir(), "a.txt"), []byte("a"))
	var closed int
	v := h.view(func(c *Config) {
		c.OnClose = func() { closed++ }
	})
	if v.Key(xkb.KeySpace) {
		t.Error("a view showing nothing took Space")
	}
	h.show(v, note)
	for _, k := range []xkb.Keysym{xkb.KeySpace, xkb.KeyEscape} {
		if !v.Key(k) {
			t.Errorf("the view left key %#x alone", k)
		}
	}
	for _, k := range []xkb.Keysym{xkb.KeyLeft, xkb.KeyUp, xkb.KeyRight, xkb.KeyDown, xkb.Keya} {
		if v.Key(k) {
			t.Errorf("the view took key %#x, which is the host's", k)
		}
	}
	if closed != 2 {
		t.Errorf("closed %d times, want 2", closed)
	}
	bare := h.view(nil)
	h.show(bare, note)
	if bare.Key(xkb.KeySpace) || bare.Key(xkb.KeyRight) {
		t.Error("a host without callbacks lost its keys")
	}
}

// The Open button opens the file shown through the host; a host that
// opens nothing gets no button.
func TestTheOpenButtonOpensTheFileShown(t *testing.T) {
	h := newHarness(t)
	note := writeFile(t, filepath.Join(t.TempDir(), "a.txt"), []byte("a"))
	var opened []string
	v := h.view(func(c *Config) { c.Open = func(p string) { opened = append(opened, p) } })
	button := treetest.Button(t, v.Root(), i18n.T("preview-open"))
	if button.Visible() {
		t.Error("the Open button shows with nothing to open")
	}
	h.show(v, note)
	if !button.Visible() {
		t.Fatal("the Open button hides while a file shows")
	}
	button.OnClick()
	if len(opened) != 1 || opened[0] != note {
		t.Errorf("opened %v, want %s", opened, note)
	}
	if bare := h.view(nil); len(treetest.All[*widget.Button](bare.Root())) != 0 {
		t.Error("a host that opens nothing got a button")
	}
}

// A filling view gives a picture, a video and text all the space it is
// given, past its boxes, and keeps an icon and a message in its middle.
func TestAFillingViewGivesWhatItShowsAllItsSpace(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	v := h.view(func(c *Config) { c.Fill = true })
	whole := func() { v.Root().Arrange(render.Rect{W: 1200, H: 900}) }
	for _, c := range []struct {
		path   string
		widget func() widget.Boundser
	}{
		{writeFile(t, filepath.Join(dir, "pano.png"), pngData(t, gradient(800, 200))), func() widget.Boundser { return v.Picture() }},
		{writeFile(t, filepath.Join(dir, "clip.y4m"), y4mClip(64, 32, 2)), func() widget.Boundser { return v.video }},
		{writeFile(t, filepath.Join(dir, "note.txt"), []byte("note")), func() widget.Boundser { return v.code }},
	} {
		h.show(v, c.path)
		whole()
		got, room := c.widget().Bounds(), v.visual.Bounds()
		if got != room || room.W < 1000 || room.H < 600 {
			t.Errorf("%s shows at %+v in a view of %+v, want all of a view filling most of 1200x900", filepath.Base(c.path), got, room)
		}
	}
	h.show(v, writeFile(t, filepath.Join(dir, "bad.png"), []byte("\x89PNG\r\n\x1a\n")))
	whole()
	if v.Renderer() != Failed {
		t.Fatalf("bad.png shows as %v", v.Renderer())
	}
	if b := v.visual.Bounds(); b.H > 300 || b.Y < 200 {
		t.Errorf("the failure's icon and message sit at %+v, want them in the middle of 1200x900", b)
	}
}

// The host hears what a file shows as once its load lands, and again
// when its video fails as it plays; it reaches the picture and the
// stream shown, and neither once another file shows.
func TestTheHostHearsWhatAFileShowsAsAndReachesIt(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	note := writeFile(t, filepath.Join(dir, "note.txt"), []byte("note"))
	clip := writeFile(t, filepath.Join(dir, "clip.y4m"), y4mClip(64, 32, 2))
	pic := writeFile(t, filepath.Join(dir, "pic.png"), pngData(t, gradient(40, 30)))
	var heard []Renderer
	v := h.view(func(c *Config) { c.OnRender = func(r Renderer) { heard = append(heard, r) } })
	h.show(v, note)
	if v.Stream() != nil {
		t.Error("text has a stream")
	}
	h.show(v, clip)
	stream := v.Stream()
	if stream == nil || stream != v.video.Stream() {
		t.Fatalf("the video plays %v, the host reaches %v", v.video.Stream(), stream)
	}
	stream.Fail(errors.New("no decoder for this codec"))
	h.until("the failed video", func() bool { return v.Renderer() == Failed })
	if v.Stream() != nil {
		t.Error("the host still reaches the failed video's stream")
	}
	h.show(v, pic)
	lay(v.Root())
	if views := treetest.All[*widget.ImageView](v.Root()); len(views) != 1 || views[0] != v.Picture() || v.Picture().Bounds().Empty() {
		t.Errorf("the host reaches %p, the picture shows in %v", v.Picture(), views)
	}
	want := []Renderer{Text, Video, Failed, Image}
	if !slices.Equal(heard, want) {
		t.Errorf("the host heard %v, want %v", heard, want)
	}
	h.show(v, note)
	if heard[len(heard)-1] != Text || len(heard) != len(want)+1 {
		t.Errorf("the host heard %v after the note", heard)
	}
}

// The picture types are the types shown as a picture, and a caller
// changing its copy changes nothing.
func TestThePictureTypesAreTheTypesShownAsAPicture(t *testing.T) {
	db := testDB(t)
	types := PictureTypes()
	for _, mimeType := range types {
		if got := RendererFor(db, mimeType); got != Image {
			t.Errorf("RendererFor(%s) = %v, want image", mimeType, got)
		}
	}
	types[0] = "application/x-thing"
	if PictureTypes()[0] == "application/x-thing" || RendererFor(db, "application/x-thing") == Image {
		t.Error("changing the copy changed the picture types")
	}
}
