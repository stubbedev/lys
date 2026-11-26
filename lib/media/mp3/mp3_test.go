package mp3

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/media/demux"
	"github.com/stubbedev/lys/lib/media/internal/mpa"
	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

// coding is what a stream's frames use, counted frame by frame and
// granule by granule.
type coding struct {
	midSide, intensity, crc, short, mixed, startStop int
}

// The fixtures, testdata/SOURCE.txt says how they were made; each has
// FFmpeg 9.0.2's 16-bit decode beside it, trimmed as a LAME tag says.
// uses is what each must exercise.
var fixtures = []struct {
	name     string
	version  mpa.Version
	rate     int
	channels int
	// gapless is set where a LAME tag names the encoder's delay and
	// padding.
	gapless bool
	uses    coding
}{
	{"mpeg1-joint.mp3", mpa.MPEG1, 44100, 2, true, coding{midSide: 1, short: 1, startStop: 1}},
	{"mpeg1-vbr-mono.mp3", mpa.MPEG1, 48000, 1, true, coding{short: 1}},
	{"mpeg1-stereo-crc.mp3", mpa.MPEG1, 32000, 2, true, coding{crc: 1}},
	{"mpeg2-joint.mp3", mpa.MPEG2, 22050, 2, true, coding{midSide: 1, short: 1, startStop: 1}},
	{"mpeg2-mono.mp3", mpa.MPEG2, 16000, 1, false, coding{}},
	{"mpeg25-stereo.mp3", mpa.MPEG25, 11025, 2, true, coding{}},
	{"mpeg25-mono.mp3", mpa.MPEG25, 8000, 1, false, coding{}},
	{"intensity-mixed.mp3", mpa.MPEG1, 44100, 2, false, coding{midSide: 1, intensity: 1, short: 1, mixed: 1, startStop: 1}},
	{"intensity-lsf.mp3", mpa.MPEG2, 22050, 2, false, coding{midSide: 1, intensity: 1, short: 1, mixed: 1, startStop: 1}},
}

func fixture(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

// packets reads the first track and every packet of a file.
func packets(t testing.TB, path string) (demux.Track, [][]byte) {
	t.Helper()
	f, err := demux.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var all [][]byte
	for {
		p, err := f.ReadPacket()
		if errors.Is(err, io.EOF) {
			return f.Tracks()[0], all
		}
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, bytes.Clone(p.Data))
	}
}

func samplesIn(d time.Duration, rate int) int {
	return int((int64(d)*int64(rate) + int64(time.Second)/2) / int64(time.Second))
}

// decodeTrimmed decodes every packet of a file and trims the decode as
// the track's delay and duration say.
func decodeTrimmed(t testing.TB, path string) (demux.Track, []int16) {
	t.Helper()
	track, all := packets(t, path)
	d, err := NewDecoder(track.SampleRate, track.Channels)
	if err != nil {
		t.Fatal(err)
	}
	var pcm []int16
	for i, p := range all {
		if pcm, err = d.Decode(pcm, p); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}
	from := samplesIn(track.CodecDelay, track.SampleRate) * track.Channels
	n := samplesIn(track.Duration, track.SampleRate) * track.Channels
	if from+n > len(pcm) {
		t.Fatalf("%d samples decoded, the track has %d after %d of delay", len(pcm), n, from)
	}
	return track, pcm[from : from+n]
}

func readPCM(t testing.TB, path string) []int16 {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := make([]int16, len(b)/2)
	for i := range s {
		s[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return s
}

// compare is the SNR of got against want in dB and the largest
// difference between two samples.
func compare(want, got []int16) (snr float64, worst int) {
	var sig, noise float64
	for i, w := range want {
		e := int(got[i]) - int(w)
		worst = max(worst, e, -e)
		sig += float64(w) * float64(w)
		noise += float64(e) * float64(e)
	}
	return 10 * math.Log10(sig/max(noise, 1e-9)), worst
}

// usage counts what the frames of a file use, through the decoder's
// own reading of their side information.
func usage(t *testing.T, path string) coding {
	t.Helper()
	track, all := packets(t, path)
	d, err := NewDecoder(track.SampleRate, track.Channels)
	if err != nil {
		t.Fatal(err)
	}
	var c coding
	for _, p := range all {
		h, err := mpa.Parse(p)
		if err != nil {
			t.Fatal(err)
		}
		m := frameMode{
			mpeg1:     h.Version == mpa.MPEG1,
			midSide:   h.Mode == mpa.JointStereo && h.ModeExt&2 != 0,
			intensity: h.Mode == mpa.JointStereo && h.ModeExt&1 != 0,
			channels:  h.Channels(),
		}
		body := p[mpa.HeaderSize:]
		if h.Protected {
			c.crc++
			body = body[crcSize:]
		}
		if m.midSide {
			c.midSide++
		}
		if m.intensity {
			c.intensity++
		}
		if _, err := d.readSideInfo(&bitReader{b: body[:h.SideInfoSize()]}, m, len(body)-h.SideInfoSize()); err != nil {
			t.Fatal(err)
		}
		granules := 1
		if m.mpeg1 {
			granules = 2
		}
		for _, g := range d.granules[:granules*m.channels] {
			switch {
			case g.blockType == shortBlock && g.mixed:
				c.mixed++
			case g.blockType == shortBlock:
				c.short++
			case g.blockType != 0:
				c.startStop++
			}
		}
	}
	return c
}

func TestDecodeMatchesFFmpeg(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			path := fixture(t, c.name)
			track, got := decodeTrimmed(t, path)
			if track.SampleRate != c.rate || track.Channels != c.channels {
				t.Fatalf("%d channels at %d Hz, want %d at %d", track.Channels, track.SampleRate, c.channels, c.rate)
			}
			if h, err := mpa.Parse(packetsOf(t, path)[0]); err != nil || h.Version != c.version {
				t.Fatalf("%v (%v), want %v", h.Version, err, c.version)
			}
			if gapless := track.CodecDelay > 0; gapless != c.gapless {
				t.Errorf("delay %v: a LAME tag %v, want %v", track.CodecDelay, gapless, c.gapless)
			}
			want := readPCM(t, path[:len(path)-len(".mp3")]+".pcm")
			if len(got) != len(want) {
				t.Fatalf("%d samples, FFmpeg's decode has %d", len(got), len(want))
			}
			snr, worst := compare(want, got)
			t.Logf("SNR %.1f dB against FFmpeg, samples up to %d apart", snr, worst)
			if snr < 90 || worst > 2 {
				t.Errorf("SNR %.1f dB against FFmpeg, samples up to %d apart: want at least 90 dB and 2", snr, worst)
			}
		})
	}
}

func packetsOf(t testing.TB, path string) [][]byte {
	t.Helper()
	_, all := packets(t, path)
	return all
}

// Each fixture codes what it is there to test.
func TestTheFixturesCodeWhatTheyTest(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			got := usage(t, fixture(t, c.name))
			for _, f := range []struct {
				what      string
				want, got int
			}{
				{"mid/side frames", c.uses.midSide, got.midSide},
				{"intensity frames", c.uses.intensity, got.intensity},
				{"frames with a CRC", c.uses.crc, got.crc},
				{"short blocks", c.uses.short, got.short},
				{"mixed blocks", c.uses.mixed, got.mixed},
				{"start and stop blocks", c.uses.startStop, got.startStop},
			} {
				if f.want > 0 && f.got == 0 {
					t.Errorf("no %s: %+v", f.what, got)
				}
			}
			t.Logf("%+v", got)
		})
	}
}

// decodeAll decodes every packet with one decoder.
func decodeAll(t testing.TB, d *Decoder, all [][]byte) []int16 {
	t.Helper()
	var pcm []int16
	for i, p := range all {
		var err error
		if pcm, err = d.Decode(pcm, p); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
	}
	return pcm
}

func newDecoder(t testing.TB, track demux.Track) *Decoder {
	t.Helper()
	d, err := NewDecoder(track.SampleRate, track.Channels)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// The frames of an MPEG audio file copied into Matroska and MP4 decode
// to the same samples, and so do the frames run together in packets
// of several.
func TestFramesDecodeAlikeInEveryContainerAndPacking(t *testing.T) {
	track, frames := packets(t, fixture(t, "mpeg1-joint.mp3"))
	want := decodeAll(t, newDecoder(t, track), frames)
	for _, name := range []string{"mpeg1-joint.mka", "mpeg1-joint.mp4"} {
		ct, all := packets(t, fixture(t, name))
		if ct.Codec != demux.MP3 || ct.SampleRate != track.SampleRate || ct.Channels != track.Channels {
			t.Fatalf("%s: track %+v", name, ct)
		}
		if got := decodeAll(t, newDecoder(t, ct), all); !slices.Equal(got, want) {
			t.Errorf("%s decodes to %d samples unlike the MPEG audio file's %d", name, len(got), len(want))
		}
	}
	var runs [][]byte
	for i := 0; i < len(frames); i += 3 {
		runs = append(runs, slices.Concat(frames[i:min(i+3, len(frames))]...))
	}
	if got := decodeAll(t, newDecoder(t, track), runs); !slices.Equal(got, want) {
		t.Errorf("packets of three frames decode to %d samples unlike one frame a packet's %d", len(got), len(want))
	}
}

// After Reset a decoder starts over at any frame: a frame whose data
// begins in the frames before it is silence, and from the next frame
// on that it decodes whole, the output is the one decoding from the
// start gives, once the overlap of the frames missed has passed.
func TestResetDecodesFromAnyFrame(t *testing.T) {
	track, frames := packets(t, fixture(t, "mpeg1-joint.mp3"))
	d := newDecoder(t, track)
	want := decodeAll(t, d, frames)
	const from = 5
	d.Reset()
	got := decodeAll(t, d, frames[from:])
	perFrame := len(want) / len(frames)
	if len(got) != perFrame*(len(frames)-from) {
		t.Fatalf("%d samples from frame %d on, want %d", len(got), from, perFrame*(len(frames)-from))
	}
	settled := 2 * perFrame
	if snr, worst := compare(want[from*perFrame+settled:], got[settled:]); worst != 0 {
		t.Errorf("after a reset and two frames, samples up to %d apart (SNR %.1f dB), want none", worst, snr)
	}
}

func TestDecodeRejectsWhatItCannotDecode(t *testing.T) {
	track, frames := packets(t, fixture(t, "mpeg1-joint.mp3"))
	frame := frames[3]
	d := newDecoder(t, track)
	dst := []int16{7}
	for _, c := range []struct {
		name   string
		packet []byte
		want   error
	}{
		{"an empty packet", nil, ErrInvalidFrame},
		{"no frame header", []byte("not an MPEG audio frame"), ErrInvalidFrame},
		{"a frame cut short", frame[:len(frame)-1], ErrInvalidFrame},
		{"a frame and a cut one", slices.Concat(frame, frame[:10]), ErrInvalidFrame},
		{"a Layer II frame", layer2Frame(t), ErrUnsupported},
		{"a 48 kHz frame in a 44.1 kHz stream", withHeader(frame, frame[2]&^0x0c|0x04, frame[3]), ErrInvalidFrame},
		{"a mono frame in a stereo stream", withHeader(frame, frame[2], frame[3]|0xc0), ErrInvalidFrame},
		{"more big values than a granule holds", withBigValues(frame, 289), ErrInvalidFrame},
		{"granules longer than their data", withPart23(frame, 4095), ErrInvalidFrame},
	} {
		got, err := d.Decode(dst, c.packet)
		if !errors.Is(err, c.want) || !slices.Equal(got, dst) {
			t.Errorf("%s: %d samples, %v; want dst as passed and %v", c.name, len(got), err, c.want)
		}
	}
	for _, c := range []struct{ rate, channels int }{{44000, 2}, {44100, 0}, {44100, 3}} {
		if _, err := NewDecoder(c.rate, c.channels); err == nil {
			t.Errorf("a decoder of %d channels at %d Hz", c.channels, c.rate)
		}
	}
}

// layer2Frame is a frame of silence in MPEG-1 Layer II at 160 kbit/s
// and 44.1 kHz, stereo.
func layer2Frame(t testing.TB) []byte {
	t.Helper()
	header := []byte{0xff, 0xfd, 0x90, 0x00}
	h, err := mpa.Parse(header)
	if err != nil {
		t.Fatal(err)
	}
	f := make([]byte, h.Size())
	copy(f, header)
	return f
}

func withHeader(frame []byte, b2, b3 byte) []byte {
	f := bytes.Clone(frame)
	f[2], f[3] = b2, b3
	return f
}

// sideInfo rewrites the first granule's side information of an MPEG-1
// stereo frame without a CRC through set.
func sideInfo(frame []byte, set func(w *bitWriter, r *bitReader)) []byte {
	f := bytes.Clone(frame)
	r := &bitReader{b: f[mpa.HeaderSize : mpa.HeaderSize+32]}
	var w bitWriter
	w.put(r.get(20), 20)
	set(&w, r)
	for r.pos < 32*8 {
		w.put(r.get(1), 1)
	}
	copy(f[mpa.HeaderSize:], w.b)
	return f
}

func withBigValues(frame []byte, n int) []byte {
	return sideInfo(frame, func(w *bitWriter, r *bitReader) {
		w.put(r.get(12), 12)
		r.get(9)
		w.put(n, 9)
	})
}

func withPart23(frame []byte, n int) []byte {
	return sideInfo(frame, func(w *bitWriter, r *bitReader) {
		r.get(12)
		w.put(n, 12)
	})
}

func TestDecodeAllocatesNothing(t *testing.T) {
	testbuild.SkipAllocBudget(t)
	track, frames := packets(t, fixture(t, "mpeg1-joint.mp3"))
	d := newDecoder(t, track)
	dst := make([]int16, 0, 1152*track.Channels)
	i := 0
	if allocs := testing.AllocsPerRun(len(frames)-1, func() {
		if _, err := d.Decode(dst[:0], frames[i]); err != nil {
			t.Fatal(err)
		}
		i++
	}); allocs != 0 {
		t.Errorf("Decode allocates %.1f times per frame, want 0", allocs)
	}
}

// Every table of scale factor band widths covers a granule's 576
// values, so a spectrum's decoding ends with the table.
func TestTheBandTablesCoverAGranule(t *testing.T) {
	for set := range scfLong {
		for _, c := range []struct {
			name  string
			bands []uint8
		}{{"long", scfLong[set][:]}, {"short", scfShort[set][:]}, {"mixed", scfMixed[set][:]}} {
			sum := 0
			for _, w := range c.bands {
				sum += int(w)
			}
			if sum != granuleSize {
				t.Errorf("%s bands of set %d cover %d values", c.name, set, sum)
			}
		}
	}
}

// A mixed block's long bands are as many values as the subbands its
// transform takes as long, at every rate.
func TestMixedBlocksSplitWhereTheTransformDoes(t *testing.T) {
	for _, rate := range []int{8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000} {
		d, err := NewDecoder(rate, 2)
		if err != nil {
			t.Fatal(err)
		}
		bands := d.t.mixed[d.bandSet]
		long := 0
		for _, w := range bands[:mixedLongBands(rate >= 32000)] {
			long += int(w)
		}
		short := 0
		for _, w := range bands[mixedLongBands(rate >= 32000):] {
			short += int(w)
		}
		if want := d.mixedLongSubbands() * 18; long != want || long+short != granuleSize {
			t.Errorf("%d Hz: %d values of long bands and %d of short, want %d and %d", rate, long, short, want, granuleSize-want)
		}
	}
}

func FuzzDecode(f *testing.F) {
	for _, c := range fixtures {
		_, frames := packets(f, fixture(f, c.name))
		f.Add(frames[0])
		f.Add(slices.Concat(frames[1:4]...))
	}
	f.Fuzz(func(t *testing.T, packet []byte) {
		h, err := mpa.Parse(packet)
		if err != nil {
			return
		}
		d, err := NewDecoder(h.SampleRate, h.Channels())
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			pcm, err := d.Decode(nil, packet)
			if err != nil {
				return
			}
			if len(pcm)%(h.Samples()*h.Channels()) != 0 {
				t.Fatalf("%d samples from frames of %d", len(pcm), h.Samples()*h.Channels())
			}
		}
	})
}

func BenchmarkDecode(b *testing.B) {
	track, frames := packets(b, fixture(b, "mpeg1-joint.mp3"))
	d := newDecoder(b, track)
	dst := make([]int16, 0, 1152*track.Channels)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if _, err := d.Decode(dst[:0], frames[i%len(frames)]); err != nil {
			b.Fatal(err)
		}
	}
}
