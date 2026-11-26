package flac

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/media/demux"
	"github.com/stubbedev/lys/lib/media/internal/flacstream"
	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

// The fixtures, testdata/SOURCE.txt says how they were made: stereo
// coded with each channel assignment, six 24-bit channels coded with
// fixed predictors, and a 24-bit stream of 16-bit samples, its low
// bits wasted. s16 is the MD5 of FFmpeg 9.0.2's decode to 16 bits.
var fixtures = []struct {
	name     string
	channels int
	bps      int
	s16      string
}{
	{"indep.flac", 2, 16, "92396a7d65e53e6d8b0ffd761b7ab385"},
	{"left_side.flac", 2, 16, "92396a7d65e53e6d8b0ffd761b7ab385"},
	{"right_side.flac", 2, 16, "92396a7d65e53e6d8b0ffd761b7ab385"},
	{"mid_side.flac", 2, 16, "92396a7d65e53e6d8b0ffd761b7ab385"},
	{"surround24.flac", 6, 24, "49c7677bb5b73244eb1c3dbabc51507a"},
	{"wasted24.flac", 1, 24, "ebcc79dd4a058f401a00c29685f9ddb1"},
}

func frames(t testing.TB, path string) (demux.Track, [][]byte) {
	t.Helper()
	f, err := demux.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var packets [][]byte
	for {
		p, err := f.ReadPacket()
		if errors.Is(err, io.EOF) {
			return f.Tracks()[0], packets
		}
		if err != nil {
			t.Fatal(err)
		}
		packets = append(packets, bytes.Clone(p.Data))
	}
}

// streamMD5 decodes every frame at the stream's depth and hashes the
// samples as STREAMINFO's MD5 does: interleaved, little-endian, in
// whole bytes.
func streamMD5(t *testing.T, d *Decoder, packets [][]byte) []byte {
	t.Helper()
	h := md5.New()
	width := (d.info.BitsPerSample + 7) / 8
	var buf []byte
	for i, p := range packets {
		n, _, err := d.decode(p)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		buf = buf[:0]
		for s := range n {
			for ch := range d.info.Channels {
				v := uint64(d.chans[ch][s])
				for b := range width {
					buf = append(buf, byte(v>>(8*b)))
				}
			}
		}
		h.Write(buf)
	}
	return h.Sum(nil)
}

func TestDecodeMatchesTheEncodersMD5(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			track, packets := frames(t, filepath.Join("testdata", c.name))
			d, err := NewDecoder(track.Config)
			if err != nil {
				t.Fatal(err)
			}
			if d.Channels() != c.channels || d.BitsPerSample() != c.bps {
				t.Fatalf("%d channels of %d bits, want %d of %d", d.Channels(), d.BitsPerSample(), c.channels, c.bps)
			}
			want := track.Config[18:]
			if got := streamMD5(t, d, packets); !bytes.Equal(got, want) {
				t.Errorf("MD5 %x, STREAMINFO's %x", got, want)
			}
		})
	}
}

func decodeAll(t *testing.T, d *Decoder, packets [][]byte) []int16 {
	t.Helper()
	var pcm []int16
	for i, p := range packets {
		var err error
		if pcm, err = d.Decode(pcm, p); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	return pcm
}

func md5Of(pcm []int16) string {
	b := make([]byte, 2*len(pcm))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

func TestDecodeTo16BitsAsFFmpegDoes(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			track, packets := frames(t, filepath.Join("testdata", c.name))
			d, err := NewDecoder(track.Config)
			if err != nil {
				t.Fatal(err)
			}
			if got := md5Of(decodeAll(t, d, packets)); got != c.s16 {
				t.Errorf("16-bit PCM MD5 %s, FFmpeg's %s", got, c.s16)
			}
		})
	}
}

// The same stream decodes the same out of every container that carries
// FLAC.
func TestDecodeEveryContainer(t *testing.T) {
	var want []int16
	for _, name := range []string{"tone.flac", "flac.oga", "flac.mp4", "flac.mka"} {
		track, packets := frames(t, filepath.Join("..", "demux", "testdata", name))
		d, err := NewDecoder(track.Config)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		pcm := decodeAll(t, d, packets)
		if len(pcm) != 8000 {
			t.Errorf("%s: %d samples, want the second's 8000", name, len(pcm))
		}
		if want != nil && !slices.Equal(pcm, want) {
			t.Errorf("%s decodes otherwise than tone.flac", name)
		}
		want = pcm
	}
}

func TestDecodeRejects(t *testing.T) {
	track, packets := frames(t, filepath.Join("testdata", "mid_side.flac"))
	d, err := NewDecoder(track.Config)
	if err != nil {
		t.Fatal(err)
	}
	frame := packets[1]
	flipped := bytes.Clone(frame)
	flipped[len(flipped)/2] ^= 1
	header, err := flacstream.ParseHeader(frame)
	if err != nil {
		t.Fatal(err)
	}
	badHeader := bytes.Clone(frame)
	badHeader[header.Size-1] ^= 1
	for name, f := range map[string][]byte{
		"a flipped bit":        flipped,
		"a cut frame":          frame[:len(frame)-7],
		"a broken CRC-8":       badHeader,
		"no frame":             nil,
		"a header and no more": frame[:header.Size],
	} {
		if _, err := d.Decode(nil, f); !errors.Is(err, ErrInvalidFrame) {
			t.Errorf("%s: %v, want ErrInvalidFrame", name, err)
		}
	}
	mono := bytes.Clone(track.Config)
	mono[12] &^= 0x0e
	m, err := NewDecoder(mono)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Decode(nil, frame); !errors.Is(err, ErrInvalidFrame) {
		t.Errorf("a stereo frame in a mono stream: %v, want ErrInvalidFrame", err)
	}
	if _, err := NewDecoder(track.Config[:20]); !errors.Is(err, flacstream.ErrInvalid) {
		t.Errorf("a cut STREAMINFO: %v, want ErrInvalid", err)
	}
}

func TestDecodeAllocatesNothing(t *testing.T) {
	testbuild.SkipAllocBudget(t)
	track, packets := frames(t, filepath.Join("testdata", "surround24.flac"))
	d, err := NewDecoder(track.Config)
	if err != nil {
		t.Fatal(err)
	}
	dst := make([]int16, 0, d.info.MaxBlockSize*d.Channels())
	i := 0
	if allocs := testing.AllocsPerRun(len(packets)-1, func() {
		if _, err := d.Decode(dst[:0], packets[i]); err != nil {
			t.Fatal(err)
		}
		i++
	}); allocs != 0 {
		t.Errorf("Decode allocates %.1f times per frame, want 0", allocs)
	}
}

func FuzzDecode(f *testing.F) {
	var configs [][]byte
	for _, c := range fixtures {
		track, packets := frames(f, filepath.Join("testdata", c.name))
		configs = append(configs, track.Config)
		f.Add(packets[0])
		f.Add(packets[len(packets)-1])
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		for _, cfg := range configs {
			d, err := NewDecoder(cfg)
			if err != nil {
				t.Fatal(err)
			}
			pcm, err := d.Decode(nil, frame)
			if err != nil {
				continue
			}
			h, herr := flacstream.ParseHeader(frame)
			if herr != nil || len(pcm) != h.BlockSize*d.Channels() {
				t.Fatalf("%d samples from a frame whose header is %+v, %v", len(pcm), h, herr)
			}
		}
	})
}

func BenchmarkDecode(b *testing.B) {
	track, packets := frames(b, filepath.Join("testdata", "indep.flac"))
	d, err := NewDecoder(track.Config)
	if err != nil {
		b.Fatal(err)
	}
	dst := make([]int16, 0, d.info.MaxBlockSize*d.Channels())
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := d.Decode(dst[:0], packets[i%len(packets)]); err != nil {
			b.Fatal(err)
		}
	}
}
