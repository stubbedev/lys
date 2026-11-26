package mp3

import (
	"bytes"
	"math/rand/v2"
	"os"
	"testing"
)

// bitWriter appends bits most significant first.
type bitWriter struct {
	b []byte
	n int
}

func (w *bitWriter) put(v, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.n%8 == 0 {
			w.b = append(w.b, 0)
		}
		w.b[len(w.b)-1] |= byte(v>>i&1) << (7 - w.n%8)
		w.n++
	}
}

// The scale factor partitions of the standard: MPEG-1's two lengths
// over long bands 0-10 and 11-20, short bands 0-5 and 6-11 of each
// window, and a mixed block's 8 long bands and short bands 3-5 then
// 6-11; MPEG-2's first row of each table, for a left channel
// (scalefac_compress below 400) and an intensity-coded right one (its
// half below 180), long, short and mixed.
var (
	mpeg1Partitions = [...][2]int{long: {11, 10}, short: {18, 18}, mixed: {17, 18}}
	mpeg1Slen       = [16][2]int{
		{0, 0},
		{0, 1},
		{0, 2},
		{0, 3},
		{3, 0},
		{1, 1},
		{1, 2},
		{1, 3},
		{2, 1},
		{2, 2},
		{2, 3},
		{3, 1},
		{3, 2},
		{3, 3},
		{4, 2},
		{4, 3},
	}
	lsfPartitions = [2][3][4]int{
		{long: {6, 5, 5, 5}, short: {9, 9, 9, 9}, mixed: {6, 9, 9, 9}},
		{long: {7, 7, 7, 0}, short: {12, 12, 12, 0}, mixed: {6, 15, 12, 0}},
	}
)

type blockKind int

const (
	long blockKind = iota
	short
	mixed
)

// synthGranule is one channel's granule of a synthetic stream: its
// spectrum is quadruples of zeros and ones only, no big values, coded
// with count1 table B, so writing it needs no Huffman code but that
// table's fixed four bits.
type synthGranule struct {
	blockType        int
	kind             blockKind
	globalGain       int
	scalefacCompress int
	subblockGain     [3]int
	scf              []int
	// quads are the flags of each quadruple, v first, from the lowest
	// frequency up.
	quads []int
	signs *rand.Rand
}

// sizes are the bit lengths and counts of the granule's scale factors.
func (g *synthGranule) sizes(mpeg1, ist bool) (lengths, counts []int) {
	if mpeg1 {
		p, s := mpeg1Partitions[g.kind], mpeg1Slen[g.scalefacCompress]
		return s[:], p[:]
	}
	sfc := g.scalefacCompress
	var l [4]int
	if ist {
		sfc >>= 1
		l = [4]int{sfc / 36, sfc % 36 / 6, sfc % 6, 0}
	} else {
		l = [4]int{sfc >> 4 / 5, sfc >> 4 % 5, sfc & 15 >> 2, sfc & 3}
	}
	c := lsfPartitions[boolBit(ist)][g.kind]
	return l[:], c[:]
}

func (g *synthGranule) writeMain(w *bitWriter, mpeg1, ist bool) int {
	start := w.n
	lengths, counts := g.sizes(mpeg1, ist)
	at := 0
	for i, n := range counts {
		for range n {
			w.put(g.scf[at], lengths[i])
			at++
		}
	}
	for _, q := range g.quads {
		w.put(15-q, 4)
		for bit := 3; bit >= 0; bit-- {
			if q>>bit&1 != 0 {
				w.put(g.signs.IntN(2), 1)
			}
		}
	}
	return w.n - start
}

func (g *synthGranule) writeSide(w *bitWriter, mpeg1 bool, part23 int) {
	w.put(part23, 12)
	w.put(0, 9)
	w.put(g.globalGain, 8)
	if mpeg1 {
		w.put(g.scalefacCompress, 4)
	} else {
		w.put(g.scalefacCompress, 9)
	}
	if g.blockType == 0 {
		w.put(0, 1)
		w.put(0, 15)
		w.put(0, 4)
		w.put(0, 3)
	} else {
		w.put(1, 1)
		w.put(g.blockType, 2)
		w.put(boolBit(g.kind == mixed), 1)
		w.put(0, 10)
		for _, s := range g.subblockGain {
			w.put(s, 3)
		}
	}
	if mpeg1 {
		w.put(0, 1)
	}
	w.put(0, 1)
	w.put(1, 1)
}

func boolBit(b bool) int {
	if b {
		return 1
	}
	return 0
}

// synthStream is a joint stereo stream of granules whose block types
// take every transition into and out of a mixed block that the
// standard's windows overlap without a gap, and whose frames take
// every mode extension: intensity and mid/side together, intensity
// alone, mid/side alone and neither. The right channel sounds only in
// its lowest frequencies, so intensity codes the bands above from the
// positions in its scale factors: every value in MPEG-1, the illegal
// one too; in MPEG-2 the legal ones below 16, where FFmpeg, which reads
// every position below 16 as legal and none above, agrees with the
// standard.
func synthStream(mpeg1 bool, seed uint64) []byte {
	r := rand.New(rand.NewPCG(seed, 1))
	blocks := []struct {
		typ  int
		kind blockKind
	}{
		{0, long},
		{1, long},
		{2, mixed},
		{2, mixed},
		{3, long},
		{0, long},
		{1, long},
		{2, short},
		{2, mixed},
		{2, mixed},
		{3, long},
		{1, long},
		{2, mixed},
		{3, long},
		{0, long},
		{0, long},
	}
	granules, header, frameSize, sideSize := 2, [4]byte{0xff, 0xfb, 0xb0, 0x40}, 626, 32
	if !mpeg1 {
		granules, header, frameSize, sideSize = 1, [4]byte{0xff, 0xf3, 0x80, 0x40}, 208, 17
	}
	var out []byte
	for f := 0; f*granules < len(blocks); f++ {
		modeExt := 3 - f%4
		h := header
		h[3] |= byte(modeExt << 4)
		var side, main bitWriter
		if mpeg1 {
			side.put(0, 9)
			side.put(0, 3)
			side.put(0, 8)
		} else {
			side.put(0, 8)
			side.put(0, 2)
		}
		for g := range granules {
			b := blocks[f*granules+g]
			for ch := range 2 {
				ist := !mpeg1 && ch == 1 && modeExt&1 != 0
				sg := synthGranule{blockType: b.typ, kind: b.kind, globalGain: 170 + r.IntN(20), signs: r}
				switch {
				case mpeg1:
					sg.scalefacCompress = 5 + r.IntN(11)
				case ist:
					sg.scalefacCompress = (36*r.IntN(5)+6*r.IntN(6)+r.IntN(6))<<1 | r.IntN(2)
				default:
					sg.scalefacCompress = 16*(5*r.IntN(5)+r.IntN(5)) + 4*r.IntN(4) + r.IntN(4)
				}
				if b.typ == 2 {
					sg.subblockGain = [3]int{r.IntN(8), r.IntN(8), r.IntN(8)}
				}
				lengths, counts := sg.sizes(mpeg1, ist)
				for i, n := range counts {
					values := 1 << lengths[i]
					if ist {
						values = max(1, min(values-1, 16))
					}
					for range n {
						sg.scf = append(sg.scf, r.IntN(values))
					}
				}
				quads := 60 + r.IntN(40)
				if !mpeg1 {
					quads = 20 + r.IntN(15)
				}
				if ch == 1 {
					quads = 3 + r.IntN(10)
				}
				for range quads {
					sg.quads = append(sg.quads, r.IntN(16))
				}
				sg.writeSide(&side, mpeg1, sg.writeMain(&main, mpeg1, ist))
			}
		}
		frame := make([]byte, frameSize)
		copy(frame, h[:])
		if copy(frame[4:], side.b) != sideSize || 4+sideSize+len(main.b) > frameSize {
			panic("a synthetic frame does not fit")
		}
		copy(frame[4+sideSize:], main.b)
		out = append(out, frame...)
	}
	return out
}

// synthFixtures are the streams synthStream writes, with FFmpeg's
// decode beside them in testdata.
var synthFixtures = []struct {
	name  string
	mpeg1 bool
	seed  uint64
}{
	{"intensity-mixed.mp3", true, 11},
	{"intensity-lsf.mp3", false, 12},
}

func TestTheSyntheticFixturesAreWhatTheWriterWrites(t *testing.T) {
	for _, c := range synthFixtures {
		path := fixture(t, c.name)
		want := synthStream(c.mpeg1, c.seed)
		if os.Getenv("LYS_MP3_REWRITE") != "" {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not what synthStream writes: rerun with LYS_MP3_REWRITE=1 and decode it with FFmpeg again", path)
		}
	}
}
