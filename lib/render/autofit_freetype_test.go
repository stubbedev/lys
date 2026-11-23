package render

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/lib/clib"
	"github.com/stubbedev/lys/lib/render/internal/autofit"
	"github.com/stubbedev/lys/lib/testbuild"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
)

const (
	ftLoadNoBitmap      = 1 << 3
	ftLoadForceAutohint = 1 << 5
	ftLoadTargetLight   = 1 << 16

	ftFaceUnitsPerEM = 136
	ftFaceGlyph      = 152
	ftSlotGlyphIndex = 24
	ftSlotOutline    = 200
	ftOutlineNPoints = 2
	ftOutlinePoints  = 8
)

type freetype struct {
	library     uintptr
	newFace     func(library uintptr, data *byte, size int64, index int64, face *unsafe.Pointer) int32
	doneFace    func(face unsafe.Pointer) int32
	setCharSize func(face unsafe.Pointer, w, h int64, hres, vres uint32) int32
	loadGlyph   func(face unsafe.Pointer, gid uint32, flags int32) int32
}

var (
	freetypeOnce sync.Once
	freetypeLib  *freetype
	freetypeErr  error
)

func loadFreetype(t *testing.T) *freetype {
	t.Helper()
	freetypeOnce.Do(func() {
		h, err := clib.Open(clib.SystemCandidates("libfreetype.so.6"))
		if err != nil {
			freetypeErr = err
			return
		}
		ft := &freetype{}
		var initFreetype func(library *uintptr) int32
		purego.RegisterLibFunc(&initFreetype, h, "FT_Init_FreeType")
		purego.RegisterLibFunc(&ft.newFace, h, "FT_New_Memory_Face")
		purego.RegisterLibFunc(&ft.doneFace, h, "FT_Done_Face")
		purego.RegisterLibFunc(&ft.setCharSize, h, "FT_Set_Char_Size")
		purego.RegisterLibFunc(&ft.loadGlyph, h, "FT_Load_Glyph")
		if rc := initFreetype(&ft.library); rc != 0 {
			freetypeErr = fmt.Errorf("FT_Init_FreeType: error %d", rc)
			return
		}
		freetypeLib = ft
	})
	if freetypeErr != nil {
		t.Fatalf("libfreetype, the reference the autohinter is held to: %v", freetypeErr)
	}
	return freetypeLib
}

func peek[T any](base unsafe.Pointer, offset int) T {
	return *(*T)(unsafe.Add(base, offset))
}

func (ft *freetype) hintedPoints(t *testing.T, face unsafe.Pointer, gid font.GID) []autofit.Point {
	t.Helper()
	if rc := ft.loadGlyph(face, uint32(gid), ftLoadNoBitmap|ftLoadForceAutohint|ftLoadTargetLight); rc != 0 {
		t.Fatalf("FT_Load_Glyph %d: error %d", gid, rc)
	}
	slot := peek[unsafe.Pointer](face, ftFaceGlyph)
	if got := peek[uint32](slot, ftSlotGlyphIndex); got != uint32(gid) {
		t.Fatalf("slot holds glyph %d, loaded %d: the FT_GlyphSlotRec offsets do not match this FreeType", got, gid)
	}
	outline := unsafe.Add(slot, ftSlotOutline)
	n := int(peek[uint16](outline, ftOutlineNPoints))
	points := peek[unsafe.Pointer](outline, ftOutlinePoints)
	pts := make([]autofit.Point, n)
	for i := range n {
		pts[i] = autofit.Point{X: peek[int64](points, 16*i), Y: peek[int64](points, 16*i+8)}
	}
	return pts
}

var autofitReferenceFaces = []struct {
	name string
	data func(t *testing.T) []byte
}{
	{"Cantarell", func(t *testing.T) []byte { return readTestFile(t, "testdata/Cantarell-Regular.ttf") }},
	{"Quicksand", func(t *testing.T) []byte { return readTestFile(t, "testdata/Quicksand-VF.ttf") }},
	{"Inter", func(t *testing.T) []byte {
		return readTestFile(t, "../../splash/internal/theme/assets/Inter-Regular.ttf")
	}},
	{"Go", func(*testing.T) []byte { return goregular.TTF }},
	{"Go Mono", func(*testing.T) []byte { return gomono.TTF }},
	{"WenQuanYi Micro Hei", func(t *testing.T) []byte { return readEnvFont(t, "LYS_TEST_CJK_FONT") }},
	{"Lohit Odia", func(t *testing.T) []byte { return readEnvFont(t, "LYS_TEST_INDIC_FONT") }},
}

const (
	largeFaceGlyphs = 2000
	largeFaceStride = 100
)

func comparedAt(i, glyphs int, px int64) bool {
	if glyphs <= largeFaceGlyphs {
		return true
	}
	return px == 16 || i%largeFaceStride == 0
}

func readEnvFont(t *testing.T, variable string) []byte {
	t.Helper()
	path := os.Getenv(variable)
	if path == "" {
		t.Skipf("%s unset: the dev shell points it at a font", variable)
	}
	return readTestFile(t, path)
}

func loadFirstFace(t *testing.T, data []byte) *Typeface {
	t.Helper()
	faces, err := font.ParseTTC(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	tf, err := NewTypeface(faces[0])
	if err != nil {
		t.Fatal(err)
	}
	return tf
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAutofitHintsEveryMappedGlyphAsFreeTypesLightAutohinter(t *testing.T) {
	if testbuild.Race {
		t.Skip("one goroutine of integer arithmetic: the race detector only slows the comparison")
	}
	ft := loadFreetype(t)
	for _, tc := range autofitReferenceFaces {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.data(t)
			tf := loadFirstFace(t, data)
			var face unsafe.Pointer
			if rc := ft.newFace(ft.library, &data[0], int64(len(data)), 0, &face); rc != 0 {
				t.Fatalf("FT_New_Memory_Face: error %d", rc)
			}
			defer runtime.KeepAlive(data)
			defer ft.doneFace(face)
			if got := peek[uint16](face, ftFaceUnitsPerEM); float64(got) != tf.upem {
				t.Fatalf("FreeType reads %d units per em, render %v: the FT_FaceRec offsets do not match this FreeType", got, tf.upem)
			}
			hf := tf.hints.autofit(tf.face)
			var gids []font.GID
			seen := map[font.GID]bool{}
			for it := tf.face.Cmap.Iter(); it.Next(); {
				_, gid := it.Char()
				if !seen[gid] {
					seen[gid] = true
					gids = append(gids, gid)
				}
			}
			compared, mismatched := 0, 0
			for px := int64(8); px <= 40; px++ {
				if rc := ft.setCharSize(face, 0, px*64, 72, 72); rc != 0 {
					t.Fatalf("FT_Set_Char_Size %dpx: error %d", px, rc)
				}
				for i, gid := range gids {
					if !comparedAt(i, len(gids), px) {
						continue
					}
					outline, ok := loadTestOutline(tf, gid)
					if !ok {
						continue
					}
					got, _, hinted := hf.Hint(nil, uint32(gid), outline, px*64)
					if !hinted {
						continue
					}
					want := ft.hintedPoints(t, face, gid)
					compared++
					if fmt.Sprint(got) != fmt.Sprint(want) {
						mismatched++
						if mismatched <= 5 {
							t.Errorf("glyph %d at %dpx: hinted %v, FreeType %v", gid, px, got, want)
						}
					}
				}
			}
			if compared == 0 {
				t.Fatal("no glyph was hinted")
			}
			if mismatched > 0 {
				t.Errorf("%d of %d hinted glyphs differ from FreeType", mismatched, compared)
			}
		})
	}
}
