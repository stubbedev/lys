package harfbuzz

import (
	"bytes"
	"os"
	"runtime"
	"slices"
	"testing"

	"github.com/ebitengine/purego"

	"github.com/stubbedev/lys/lib/clib"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
	"github.com/stubbedev/lys/lib/third_party/typesetting/language"
)

const (
	hbTagGSUB = 'G'<<24 | 'S'<<16 | 'U'<<8 | 'B'
	hbTagGPOS = 'G'<<24 | 'P'<<16 | 'O'<<8 | 'S'
)

type libharfbuzz struct {
	blobCreate      func(data *byte, length uint32, mode int32, user, destroy uintptr) uintptr
	faceCreate      func(blob uintptr, index uint32) uintptr
	faceDestroy     func(face uintptr)
	blobDestroy     func(blob uintptr)
	setCreate       func() uintptr
	setDestroy      func(set uintptr)
	setNext         func(set uintptr, cp *uint32) bool
	collectLookups  func(face uintptr, table uint32, scripts, languages, features *uint32, out uintptr)
	collectGlyphs   func(face uintptr, table, lookup uint32, before, input, after, output uintptr)
	wouldSubstitute func(face uintptr, lookup uint32, glyphs *uint32, n uint32, zeroContext bool) bool
	tagsFromScript  func(script uint32, language uintptr, scriptCount *uint32, scriptTags *uint32, languageCount, languageTags uintptr)
}

func loadLibharfbuzz(t *testing.T) *libharfbuzz {
	t.Helper()
	h, err := clib.Open(clib.SystemCandidates("libharfbuzz.so.0"))
	if err != nil {
		t.Fatalf("libharfbuzz, the reference the layout queries are held to: %v", err)
	}
	hb := &libharfbuzz{}
	purego.RegisterLibFunc(&hb.blobCreate, h, "hb_blob_create")
	purego.RegisterLibFunc(&hb.faceCreate, h, "hb_face_create")
	purego.RegisterLibFunc(&hb.faceDestroy, h, "hb_face_destroy")
	purego.RegisterLibFunc(&hb.blobDestroy, h, "hb_blob_destroy")
	purego.RegisterLibFunc(&hb.setCreate, h, "hb_set_create")
	purego.RegisterLibFunc(&hb.setDestroy, h, "hb_set_destroy")
	purego.RegisterLibFunc(&hb.setNext, h, "hb_set_next")
	purego.RegisterLibFunc(&hb.collectLookups, h, "hb_ot_layout_collect_lookups")
	purego.RegisterLibFunc(&hb.collectGlyphs, h, "hb_ot_layout_lookup_collect_glyphs")
	purego.RegisterLibFunc(&hb.wouldSubstitute, h, "hb_ot_layout_lookup_would_substitute")
	purego.RegisterLibFunc(&hb.tagsFromScript, h, "hb_ot_tags_from_script_and_language")
	return hb
}

func (hb *libharfbuzz) members(set uintptr) []uint32 {
	var out []uint32
	for cp := ^uint32(0); hb.setNext(set, &cp); {
		out = append(out, cp)
	}
	return out
}

func (hb *libharfbuzz) lookups(face uintptr, table uint32, scripts []tables.Tag, feature *tables.Tag) []uint32 {
	tags := make([]uint32, 0, len(scripts)+1)
	for _, s := range scripts {
		tags = append(tags, uint32(s))
	}
	tags = append(tags, 0)
	var features *uint32
	if feature != nil {
		features = &[]uint32{uint32(*feature), 0}[0]
	}
	set := hb.setCreate()
	defer hb.setDestroy(set)
	hb.collectLookups(face, table, &tags[0], nil, features, set)
	return hb.members(set)
}

func widen(lookups []uint16) []uint32 {
	out := make([]uint32, len(lookups))
	for i, l := range lookups {
		out[i] = uint32(l)
	}
	return out
}

func collected(collect func(add func(GID))) []uint32 {
	seen := map[uint32]bool{}
	collect(func(g GID) { seen[uint32(g)] = true })
	out := make([]uint32, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	slices.Sort(out)
	return out
}

func layoutTestFonts(t *testing.T) map[string][]byte {
	t.Helper()
	fonts := map[string][]byte{}
	for name, path := range map[string]string{
		"Quicksand":          "../../../render/testdata/Quicksand-VF.ttf",
		"Cantarell":          "../../../render/testdata/Cantarell-Regular.ttf",
		"Noto Sans Hebrew":   "../../../render/testdata/NotoSansHebrew-Regular.ttf",
		"Libertinus Serif":   os.Getenv("LYS_TEST_FEATURES_FONT"),
		"Lohit Odia":         os.Getenv("LYS_TEST_INDIC_FONT"),
		"WenQuanYi MicroHei": os.Getenv("LYS_TEST_CJK_FONT"),
	} {
		if path == "" {
			t.Errorf("%s: its variable is unset; the dev shell points it at the font", name)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		fonts[name] = data
	}
	return fonts
}

func TestOTScriptTagsAreLibharfbuzzs(t *testing.T) {
	hb := loadLibharfbuzz(t)
	scripts := []language.Script{0, language.Katakana_Or_Hiragana, language.Mathematical_notation}
	for _, r := range language.ScriptRanges {
		if !slices.Contains(scripts, r.Script) {
			scripts = append(scripts, r.Script)
		}
	}
	for _, script := range scripts {
		want := make([]uint32, 3)
		n := uint32(len(want))
		hb.tagsFromScript(uint32(script), 0, &n, &want[0], 0, 0)
		if got := AppendOTScriptTags(nil, script); !slices.Equal(widenTags(got), want[:n]) {
			t.Errorf("script %s: tags %v, libharfbuzz %v", script, got, want[:n])
		}
	}
}

func widenTags(tags []tables.Tag) []uint32 {
	out := make([]uint32, len(tags))
	for i, t := range tags {
		out[i] = uint32(t)
	}
	return out
}

func TestLayoutQueriesAreLibharfbuzzs(t *testing.T) {
	hb := loadLibharfbuzz(t)
	for name, data := range layoutTestFonts(t) {
		t.Run(name, func(t *testing.T) {
			faces, err := font.ParseTTC(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			face := faces[0]
			blob := hb.blobCreate(&data[0], uint32(len(data)), 0, 0, 0)
			defer hb.blobDestroy(blob)
			hbFace := hb.faceCreate(blob, 0)
			defer hb.faceDestroy(hbFace)
			defer runtime.KeepAlive(data)
			checkLookupCollection(t, hb, hbFace, hbTagGSUB, &face.GSUB.Layout)
			checkLookupCollection(t, hb, hbFace, hbTagGPOS, &face.GPOS.Layout)
			for lookup := range uint16(len(face.GSUB.Lookups) + 1) {
				out := hb.setCreate()
				hb.collectGlyphs(hbFace, hbTagGSUB, uint32(lookup), 0, 0, 0, out)
				want := hb.members(out)
				hb.setDestroy(out)
				got := collected(func(add func(GID)) { CollectGSUBOutput(&face.GSUB, lookup, add) })
				if !slices.Equal(got, want) {
					t.Errorf("GSUB lookup %d outputs %d glyphs, libharfbuzz %d", lookup, len(got), len(want))
				}
			}
			for lookup := range uint16(len(face.GPOS.Lookups) + 1) {
				in := hb.setCreate()
				hb.collectGlyphs(hbFace, hbTagGPOS, uint32(lookup), 0, in, 0, 0)
				want := hb.members(in)
				hb.setDestroy(in)
				got := collected(func(add func(GID)) { CollectGPOSInput(&face.GPOS, lookup, add) })
				if !slices.Equal(got, want) {
					t.Errorf("GPOS lookup %d positions %d glyphs, libharfbuzz %d", lookup, len(got), len(want))
				}
			}
			hbFont := NewFont(face)
			for lookup := range uint16(len(face.GSUB.Lookups) + 1) {
				for g := range uint32(face.NumGlyphs()) {
					glyph := []GID{GID(g)}
					want := hb.wouldSubstitute(hbFace, uint32(lookup), &g, 1, true)
					if got := hbFont.WouldSubstitute(lookup, glyph, true); got != want {
						t.Errorf("GSUB lookup %d would substitute glyph %d: %v, libharfbuzz %v", lookup, g, got, want)
					}
				}
			}
		})
	}
}

func checkLookupCollection(t *testing.T, hb *libharfbuzz, face uintptr, table uint32, layout *font.Layout) {
	t.Helper()
	missing := tables.Tag('z'<<24 | 'z'<<16 | 'z'<<8 | 'z')
	scripts := [][]tables.Tag{nil, {missing}}
	var all []tables.Tag
	for _, s := range layout.Scripts {
		scripts = append(scripts, []tables.Tag{s.Tag})
		all = append(all, s.Tag)
	}
	scripts = append(scripts, append(all, missing))
	features := []tables.Tag{missing}
	for _, f := range layout.Features {
		if !slices.Contains(features, f.Tag) {
			features = append(features, f.Tag)
		}
	}
	for _, s := range scripts {
		if got, want := widen(CollectAllLookups(layout, s)), hb.lookups(face, table, s, nil); !slices.Equal(got, want) {
			t.Errorf("table %x scripts %v: lookups %v, libharfbuzz %v", table, s, got, want)
		}
		for _, f := range features {
			if got, want := widen(CollectFeatureLookups(layout, s, f)), hb.lookups(face, table, s, &f); !slices.Equal(got, want) {
				t.Errorf("table %x scripts %v feature %s: lookups %v, libharfbuzz %v", table, s, f, got, want)
			}
		}
	}
}
