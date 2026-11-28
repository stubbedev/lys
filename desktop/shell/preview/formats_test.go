package preview

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	lysinter "github.com/stubbedev/lys/lib/inter"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// The richer types show as their own renderers, and only those types:
// an EPUB, though a zip, is no archive, and an xz-compressed file that
// is no tar is shown as other files are.
func TestTheRicherTypesHaveTheirRenderers(t *testing.T) {
	db := testDB(t)
	for mimeType, want := range map[string]Renderer{
		"image/svg+xml": Image, "text/markdown": Markdown, "text/csv": Table, "text/tab-separated-values": Table,
		"application/zip": Archive, "application/x-tar": Archive, "application/x-compressed-tar": Archive,
		"application/x-bzip2-compressed-tar": Archive, "application/x-xz-compressed-tar": Archive, "application/x-lzma-compressed-tar": Archive,
		"application/x-zstd-compressed-tar": Archive, "application/x-7z-compressed": Archive, "font/ttf": Font, "font/woff": Font, "font/woff2": Font,
		"application/json": Text, "application/x-desktop": Text,
		"application/epub+zip": Thumbnail, "application/x-xz": Thumbnail,
	} {
		if got := RendererFor(db, mimeType); got != want {
			t.Errorf("RendererFor(%s) = %v, want %v", mimeType, got, want)
		}
	}
}

// showIn shows the file name, written with data in a new folder, in a
// view from h and lays it out.
func showIn(t *testing.T, h *harness, v *View, name string, data []byte) string {
	t.Helper()
	path := writeFile(t, filepath.Join(t.TempDir(), name), data)
	h.show(v, path)
	lay(v.Root())
	return path
}

// An SVG drawing is drawn to fill the picture box at its aspect, small
// as it says it is, its own size in the detail line; one lys does not
// draw shows its source instead.
func TestAnSVGDrawingShowsDrawn(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	showIn(t, h, v, "badge.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 12"><rect width="24" height="12" fill="#3366ff"/></svg>`))
	views := treetest.All[*widget.ImageView](v.Root())
	if v.Renderer() != Image || len(views) != 1 {
		t.Fatalf("badge.svg shows as %v with %d image views", v.Renderer(), len(views))
	}
	if b := views[0].Bounds(); b.W != 320 || b.H != 160 {
		t.Errorf("the 24x12 drawing shows at %dx%d, want it filling 320x160", b.W, b.H)
	}
	if img := v.shown.picture.img.Bounds(); img.Dx() != 640 || img.Dy() != 320 {
		t.Errorf("the drawing is drawn at %v, want twice the box for HiDPI", img)
	}
	if info := v.info.Text(); !strings.Contains(info, " · 24 × 12 · ") {
		t.Errorf("the drawing's detail line is %q, want its own size", info)
	}
	for name, src := range map[string]string{
		"sizeless.svg": `<svg xmlns="http://www.w3.org/2000/svg"><rect width="4" height="4"/></svg>`,
		"broken.svg":   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><rect width="4"`,
	} {
		showIn(t, h, v, name, []byte(src))
		if v.Renderer() != Text || v.code.Text() != src {
			t.Errorf("%s shows as %v, want its source as text", name, v.Renderer())
		}
	}
	if held := h.src.Pictures.Held(); held != 0 {
		t.Errorf("the drawings no longer shown hold %d bytes", held)
	}
}

// Markdown's blocks: headings by level, paragraphs joined, items with
// their markers and nesting, quotes, code and tables as written, rules,
// and inline emphasis, code, links and images as markup.
func TestMarkdownLaysOutItsBlocks(t *testing.T) {
	src := "# Title *one*\n\nSome **bold** and _it_ text\nwrapped `a<b` here.\n\n" +
		"- first\n  - nested [link](https://x.org)\n3. third\n\n> quoted\n> on\n\n---\n\n```go\nfunc main() {}\n```\n" +
		"| a | b |\n|---|---|\n\n![a cat](cat.png) snake_case_name ~~gone~~ \\*kept\\* **open"
	want := []mdBlock{
		{kind: mdHeading, level: 1, markup: "Title <i>one</i>"},
		{kind: mdParagraph, markup: "Some <b>bold</b> and <i>it</i> text wrapped <tt>a&lt;b</tt> here."},
		{kind: mdItem, marker: "•", markup: "first"},
		{kind: mdItem, level: 1, marker: "•", markup: "nested <u>link</u>"},
		{kind: mdItem, marker: "3.", markup: "third"},
		{kind: mdQuote, markup: "quoted on"},
		{kind: mdRule},
		{kind: mdCode, markup: "func main() {}"},
		{kind: mdCode, markup: "| a | b |"},
		{kind: mdCode, markup: "|---|---|"},
		{kind: mdParagraph, markup: "<i>a cat</i> snake_case_name <s>gone</s> *kept* <b>open</b>"},
	}
	if got := markdownBlocks(src); !slices.Equal(got, want) {
		t.Errorf("the blocks are\n%+v\nwant\n%+v", got, want)
	}
	for _, s := range []string{"**a *b** c*", "[unclosed](", "`open", "*"} {
		for _, b := range markdownBlocks(s) {
			if _, ok := widget.ParseMarkup(b.markup); !ok {
				t.Errorf("%q gives markup %q that does not parse", s, b.markup)
			}
		}
	}
}

// A Markdown file shows laid out: its heading bold and larger, its
// items and its code; one that is not text shows its icon.
func TestAMarkdownFileShowsRendered(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	showIn(t, h, v, "README.md", []byte("# Notes\n\nA **strong** point.\n\n- one\n\n```\ncode\n```\n"))
	labels := treetest.All[*widget.RichLabel](v.Root())
	if v.Renderer() != Markdown || len(labels) != 4 {
		t.Fatalf("README.md shows as %v with %d rich labels", v.Renderer(), len(labels))
	}
	if labels[0].Markup() != "<b>Notes</b>" || labels[0].Bounds().H <= labels[1].Bounds().H {
		t.Errorf("the heading shows as %q at %v, want bold and taller than the text at %v", labels[0].Markup(), labels[0].Bounds(), labels[1].Bounds())
	}
	if labels[3].Markup() != "<tt>code</tt>" {
		t.Errorf("the code shows as %q", labels[3].Markup())
	}
	if b := v.markdown.scroll.Bounds(); b.W != 320 || b.H != 200 || labels[0].Bounds().Y != b.Y {
		t.Errorf("the Markdown shows in %v from %v, want the text box from its top", b, labels[0].Bounds())
	}
	showIn(t, h, v, "blob.md", []byte("\x00\x01binary"))
	if v.Renderer() != Icon || len(treetest.All[*widget.RichLabel](v.Root())) != 0 {
		t.Errorf("a binary .md shows as %v", v.Renderer())
	}
}

// cells are the texts of a table's header and of its rows' cells.
func cells(t *testing.T, v *View) []string {
	t.Helper()
	if v.table == nil {
		t.Fatalf("%s shows no table", v.Path())
	}
	var out []string
	for _, l := range treetest.All[*widget.Label](v.table) {
		out = append(out, l.Text())
	}
	for i := range v.table.Rows() {
		out = append(out, v.table.Row(i)...)
	}
	return out
}

// A CSV or TSV file shows as a table under its first row, quoted cells
// whole; one whose head is cut drops the row cut short; text that does
// not parse as a table shows as text.
func TestATableShowsItsRows(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	showIn(t, h, v, "people.csv", []byte("name,city\nAda,\"London, UK\"\nAlan,Wilmslow\n"))
	if v.Renderer() != Table {
		t.Fatalf("people.csv shows as %v", v.Renderer())
	}
	got := cells(t, v)
	for _, want := range []string{"name", "city", "Ada", "London, UK", "Alan", "Wilmslow"} {
		if !slices.Contains(got, want) {
			t.Errorf("the table shows %q, missing %q", got, want)
		}
	}
	if b := v.table.Bounds(); b.W != 320 || b.H != 200 {
		t.Errorf("the table is %v, want the text box", b)
	}
	if want := i18n.T("preview-table-rows", i18n.Int("count", 2), i18n.Str("shown", "2")); !strings.Contains(v.info.Text(), want) {
		t.Errorf("the table's detail line is %q, want %q", v.info.Text(), want)
	}
	showIn(t, h, v, "people.tsv", []byte("name\tcity\nAda\tLondon, UK\n"))
	if got := cells(t, v); v.Renderer() != Table || !slices.Contains(got, "London, UK") || !slices.Contains(got, "city") {
		t.Errorf("people.tsv shows as %v with %q", v.Renderer(), got)
	}

	long := bytes.Repeat([]byte("1,2\n"), TextBytes/4)
	long = append([]byte("a,b\n"), long...)
	long = append(long, "3,cut short and longer than the rest\n"...)
	showIn(t, h, v, "long.csv", long)
	if v.Renderer() != Table || !v.shown.table.more || len(v.shown.table.rows) != TextBytes/4-1 {
		t.Errorf("a cut table shows %d rows, more %v; want the rows read whole", len(v.shown.table.rows), v.shown.table.more)
	}
	if want := tableRows(v.shown.table); !strings.Contains(v.info.Text(), want) || !strings.Contains(want, "+") {
		t.Errorf("a cut table's detail line is %q, want its rows as a floor", v.info.Text())
	}
	showIn(t, h, v, "quote.csv", []byte("a,b\n\"x,y\n"))
	if v.Renderer() != Text || v.code.Text() != "a,b\n\"x,y\n" {
		t.Errorf("a table that does not parse shows as %v", v.Renderer())
	}
}

// A whole JSON file shows indented; one that does not parse, or is cut,
// shows as written.
func TestJSONShowsFormatted(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	showIn(t, h, v, "data.json", []byte(`{"a":[1,2],"b":{"c":true}}`))
	if want := "{\n  \"a\": [\n    1,\n    2\n  ],\n  \"b\": {\n    \"c\": true\n  }\n}"; v.Renderer() != Text || v.code.Text() != want {
		t.Errorf("data.json shows as %v:\n%s", v.Renderer(), v.code.Text())
	}
	showIn(t, h, v, "bad.json", []byte(`{"a":`))
	if v.code.Text() != `{"a":` {
		t.Errorf("JSON that does not parse shows as %q", v.code.Text())
	}
	cut := append([]byte(`{"a":"`), bytes.Repeat([]byte("x"), TextBytes)...)
	cut = append(cut, `"}`...)
	showIn(t, h, v, "cut.json", cut)
	if text := v.code.Text(); len(text) != TextBytes || strings.Contains(text, "\n") {
		t.Errorf("cut JSON shows %d bytes, want its head as written", len(text))
	}
	showIn(t, h, v, "app.desktop", []byte("[Desktop Entry]\nName=App\n"))
	if v.Renderer() != Text || v.code.Text() != "[Desktop Entry]\nName=App\n" {
		t.Errorf("a .desktop file, text by its type's parent, shows as %v", v.Renderer())
	}
}

// archiveFiles are what the archive fixtures hold.
var archiveFiles = []struct {
	name string
	body string
}{{"readme.txt", "hello world\n"}, {"src/main.go", "package main\n"}}

func zipOf(t *testing.T, n int) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for i := range n {
		f := archiveFiles[i%len(archiveFiles)]
		name := f.name
		if n > len(archiveFiles) {
			name = fmt.Sprintf("%d-%s", i, f.name)
		}
		fw, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func tarOf(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, f := range archiveFiles {
		if err := tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.body)), ModTime: stamp}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// compressed is data written through the compressor open makes.
func compressed[W io.WriteCloser](t *testing.T, data []byte, open func(io.Writer) (W, error)) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := open(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// An archive lists what it holds, each entry's name, size and time, and
// counts its entries in the detail line: a zip, a 7z, a tar, gzip,
// bzip2, xz, LZMA or zstd compressed; past ArchiveEntries it lists that
// many and says there are more. A damaged archive says it cannot be
// shown.
func TestAnArchiveListsItsFiles(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	fixture := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	tarball := tarOf(t)
	for name, data := range map[string][]byte{
		"code.zip": zipOf(t, 2), "code.7z": fixture("listing.7z"), "code.tar": tarball,
		"code.tar.gz":   compressed(t, tarball, func(w io.Writer) (*gzip.Writer, error) { return gzip.NewWriter(w), nil }),
		"code.tar.bz2":  fixture("listing.tar.bz2"),
		"code.tar.xz":   compressed(t, tarball, xz.NewWriter),
		"code.tar.lzma": compressed(t, tarball, lzma.NewWriter),
		"code.tar.zst":  compressed(t, tarball, func(w io.Writer) (*zstd.Encoder, error) { return zstd.NewWriter(w) }),
	} {
		showIn(t, h, v, name, data)
		if v.Renderer() != Archive {
			t.Errorf("%s shows as %v", name, v.Renderer())
			continue
		}
		got := cells(t, v)
		for _, want := range []string{i18n.T("preview-archive-name"), "readme.txt", "src/main.go", byteSize(12), modified(stamp)} {
			if !slices.Contains(got, want) {
				t.Errorf("%s lists %q, missing %q", name, got, want)
			}
		}
		if info := v.info.Text(); !strings.Contains(info, folderItems(v.shown.folder)) || v.shown.folder.items < 2 {
			t.Errorf("%s's detail line is %q, want its entries counted", name, info)
		}
	}
	showIn(t, h, v, "many.zip", zipOf(t, ArchiveEntries+1))
	if st := v.shown.folder; v.Renderer() != Archive || st.items != ArchiveEntries || !st.itemsMore || !v.shown.table.more {
		t.Errorf("a zip past the cap lists %+v as %v", st, v.Renderer())
	}
	for name, data := range map[string][]byte{
		"broken.zip": []byte("PK\x03\x04 not really"), "broken.7z": []byte("7z\xbc\xaf\x27\x1c not really, though long enough"),
		"broken.tar.xz": []byte("\xfd7zXZ\x00 not really"), "broken.tar.zst": []byte("\x28\xb5\x2f\xfd not really"),
	} {
		showIn(t, h, v, name, data)
		if v.Renderer() != Failed || v.shown.failure != Undecodable {
			t.Errorf("a damaged %s shows as %v (%v)", name, v.Renderer(), v.shown.failure)
		}
	}
}

// woffOf wraps an OpenType font in WOFF 1, each table compressed.
func woffOf(t *testing.T, sfnt []byte) []byte {
	t.Helper()
	n := int(binary.BigEndian.Uint16(sfnt[4:]))
	var tables bytes.Buffer
	dir := make([]byte, 20*n)
	offset := 44 + 20*n
	for i := range n {
		rec := sfnt[12+16*i:]
		tag, sum, off, length := rec[0:4], rec[4:8], binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		var z bytes.Buffer
		zw := zlib.NewWriter(&z)
		if _, err := zw.Write(sfnt[off : off+length]); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		data := z.Bytes()
		if len(data) >= int(length) {
			data = sfnt[off : off+length]
		}
		d := dir[20*i:]
		copy(d, tag)
		binary.BigEndian.PutUint32(d[4:], uint32(offset+tables.Len()))
		binary.BigEndian.PutUint32(d[8:], uint32(len(data)))
		binary.BigEndian.PutUint32(d[12:], length)
		copy(d[16:], sum)
		tables.Write(data)
		for tables.Len()%4 != 0 {
			tables.WriteByte(0)
		}
	}
	head := make([]byte, 44)
	copy(head, "wOFF")
	copy(head[4:], sfnt[0:4])
	binary.BigEndian.PutUint32(head[8:], uint32(offset+tables.Len()))
	binary.BigEndian.PutUint16(head[12:], uint16(n))
	binary.BigEndian.PutUint32(head[16:], uint32(len(sfnt)))
	return slices.Concat(head, dir, tables.Bytes())
}

// A font shows its specimen, set in the font itself (Inter, where the
// view's text is in Go) under its family's name, as a TTF, a WOFF and a
// WOFF2; a file that is no font shows its icon.
func TestAFontShowsItsSpecimen(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	inter, err := render.LoadFont(lysinter.Regular)
	if err != nil {
		t.Fatal(err)
	}
	width := func(face render.Font, text string, px float64) int {
		return widget.NewLabel(face, px, text, 0).Measure(widget.Row, widget.Unknown).Nat
	}
	packed, err := os.ReadFile(filepath.Join("..", "..", "..", "lib", "woff2", "testdata", "inter.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"Inter.ttf": lysinter.Regular, "Inter.woff": woffOf(t, lysinter.Regular), "Inter.woff2": packed} {
		showIn(t, h, v, name, data)
		if v.Renderer() != Font {
			t.Errorf("%s shows as %v", name, v.Renderer())
			continue
		}
		labels := treetest.All[*widget.Label](v.specimen.column)
		if len(labels) != 1+len(specimenSizes)+len(specimenLines) || labels[0].Text() != "Inter" {
			t.Fatalf("%s's specimen has %d lines, the first %q", name, len(labels), labels[0].Text())
		}
		line := specimenLines[1]
		got := labels[2+len(specimenSizes)].Measure(widget.Row, widget.Unknown).Nat
		if got != width(inter, line, specimenLinePx) || got == width(treetest.Font(t), line, specimenLinePx) {
			t.Errorf("%s sets %q %d wide, want Inter's %d", name, line, got, width(inter, line, specimenLinePx))
		}
		if labels[1].Text() != i18n.T("preview-font-sample") || labels[1].Bounds().H <= labels[len(labels)-1].Bounds().H {
			t.Errorf("%s's sample line %q is not set large", name, labels[1].Text())
		}
	}
	for name, data := range map[string]string{"fake.ttf": "not a font at all", "fake.woff2": "wOF2 not a font either, though it starts like one"} {
		showIn(t, h, v, name, []byte(data))
		if v.Renderer() != Icon || len(treetest.All[*widget.Label](v.specimen.column)) != 0 {
			t.Errorf("a %s that is no font shows as %v", name, v.Renderer())
		}
	}
}

// A sound file plays, its cover (here the placeholder) showing.
func TestASoundFilePlays(t *testing.T) {
	h := newHarness(t)
	v := h.view(nil)
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "ui", "media", "testdata", "tone.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	showIn(t, h, v, "tone.mp3", data)
	if v.Renderer() != Video || v.Stream() == nil || len(treetest.All[*widget.Video](v.Root())) != 1 {
		t.Errorf("tone.mp3 shows as %v", v.Renderer())
	}
	v.Close()
}
