package demux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/media/internal/flacstream"
)

// The audio fixtures are one second of 8 kHz mono, a 220 Hz tone that
// turns into a louder 440 Hz one at 500 ms, 24-bit in FLAC, with
// testdata/cover.png as their cover where they carry one;
// testdata/SOURCE.txt says how they were made.
var audioFixtures = []struct {
	file    string
	format  Format
	codec   Codec
	rate    int
	packets int
	cover   bool
	// timed is set where every packet's duration is known, so the
	// packets tile the stream.
	timed bool
}{
	{"vorbis.ogg", Ogg, Vorbis, 8000, 33, true, true},
	{"opus.ogg", Ogg, Opus, 48000, 51, true, true},
	{"flac.oga", Ogg, FLAC, 8000, 8, false, true},
	{"tone.flac", NativeFLAC, FLAC, 8000, 8, true, true},
	{"flac.mp4", MP4, FLAC, 8000, 8, false, true},
	{"flac.mka", Matroska, FLAC, 8000, 8, false, false},
	{"tone.mp3", MPEGAudio, MP3, 8000, 16, true, true},
}

func audioFixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func openBytes(t testing.TB, b []byte, name string) *File {
	t.Helper()
	f, err := Open(bytes.NewReader(b), int64(len(b)), name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readPackets(t *testing.T, f *File) []Packet {
	t.Helper()
	var all []Packet
	for {
		p, err := f.ReadPacket()
		if errors.Is(err, io.EOF) {
			return all
		}
		if err != nil {
			t.Fatal(err)
		}
		p.Data = bytes.Clone(p.Data)
		all = append(all, p)
	}
}

func TestReadsAudioFiles(t *testing.T) {
	cover := audioFixture(t, "cover.png")
	for _, c := range audioFixtures {
		t.Run(c.file, func(t *testing.T) {
			f := openBytes(t, audioFixture(t, c.file), c.file)
			if f.Format() != c.format {
				t.Errorf("format %v, want %v", f.Format(), c.format)
			}
			tr := f.Tracks()
			if len(tr) != 1 || tr[0].Kind != Audio || tr[0].Codec != c.codec || tr[0].SampleRate != c.rate || tr[0].Channels != 1 {
				t.Fatalf("tracks %+v", tr)
			}
			if d := f.Duration(); d < time.Second || d > time.Second+10*time.Millisecond {
				t.Errorf("duration %v, want a second", d)
			}
			if got := f.Cover(); c.cover != bytes.Equal(got, cover) || !c.cover && got != nil {
				t.Errorf("cover of %d bytes, want the fixture's: %v", len(got), c.cover)
			}
			if c.codec == FLAC {
				if info, err := flacstream.ParseStreamInfo(tr[0].Config); err != nil || info.SampleRate != c.rate || info.BitsPerSample != 24 {
					t.Errorf("Config is no STREAMINFO of the stream: %+v, %v", info, err)
				}
			}
			all := readPackets(t, f)
			if len(all) != c.packets {
				t.Fatalf("%d packets, want %d", len(all), c.packets)
			}
			if !c.timed {
				return
			}
			for i, p := range all[1:] {
				if before := all[i]; before.PTS+before.Duration != p.PTS {
					t.Errorf("packet %d ends at %v, packet %d starts at %v", i, before.PTS+before.Duration, i+1, p.PTS)
				}
			}
			last := all[len(all)-1]
			if end := last.PTS + last.Duration; end != f.Duration() && c.format != MP4 && c.format != MPEGAudio {
				t.Errorf("the packets end at %v, the file at %v", end, f.Duration())
			}
		})
	}
}

// After a seek the packets are the file's own from the last that
// starts at or before the target, a Vorbis stream's first of them
// timed as the decoder outputs it: nothing, at its end.
func TestSeekResumesAtThePacketBefore(t *testing.T) {
	for _, c := range audioFixtures {
		if c.format != Ogg && c.format != NativeFLAC && c.format != MPEGAudio {
			continue
		}
		t.Run(c.file, func(t *testing.T) {
			f := openBytes(t, audioFixture(t, c.file), c.file)
			all := readPackets(t, f)
			for _, target := range []time.Duration{550 * time.Millisecond, 200 * time.Millisecond, 0, 950 * time.Millisecond, 2 * time.Second} {
				at, err := f.Seek(target)
				if err != nil {
					t.Fatal(err)
				}
				after := readPackets(t, f)
				j := len(all) - len(after)
				if j < 0 || len(after) == 0 {
					t.Fatalf("seek to %v: %d packets after, %d in the file", target, len(after), len(all))
				}
				if at > target || all[j].PTS > max(target, 0) || all[j].PTS < at && j > 0 {
					t.Errorf("seek to %v landed at %v, resuming with the packet at %v", target, at, all[j].PTS)
				}
				for i, p := range after {
					w := all[j+i]
					timed := i > 0 || c.codec != Vorbis || j == 0
					if !bytes.Equal(p.Data, w.Data) || timed && p.PTS != w.PTS {
						t.Fatalf("seek to %v: packet %d is %d bytes at %v, the file's %d at %v", target, i, len(p.Data), p.PTS, len(w.Data), w.PTS)
					}
				}
			}
		})
	}
}

// putTotalSamples rewrites the STREAMINFO of a native FLAC file.
func putTotalSamples(b []byte, total uint64) {
	info := b[len(flacMagic)+flacstream.BlockHeaderSize:]
	packed := binary.BigEndian.Uint64(info[10:])
	binary.BigEndian.PutUint64(info[10:], packed&^(1<<36-1)|total)
}

func TestFLACStreamsOfUnknownLengthOrWithATag(t *testing.T) {
	orig := audioFixture(t, "tone.flac")
	want := readPackets(t, openBytes(t, orig, "tone.flac"))
	unknown := bytes.Clone(orig)
	putTotalSamples(unknown, 0)
	tagged := append(bytes.Clone(orig), append([]byte("TAG"), bytes.Repeat([]byte{0xff}, id3v1Size-3)...)...)
	for name, b := range map[string][]byte{"no total": unknown, "ID3v1 tag": tagged} {
		f := openBytes(t, b, name)
		if f.Duration() != time.Second {
			t.Errorf("%s: duration %v, want a second", name, f.Duration())
		}
		got := readPackets(t, f)
		if len(got) != len(want) || !bytes.Equal(got[len(got)-1].Data, want[len(want)-1].Data) {
			t.Errorf("%s: %d packets, want %d ending as the file's", name, len(got), len(want))
		}
	}
	cut := orig[:len(orig)-10]
	f := openBytes(t, cut, "cut")
	var err error
	for err == nil {
		_, err = f.ReadPacket()
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("a stream cut inside its last frame: %v, want ErrMalformed", err)
	}
}

func TestOggFailsOnBrokenPagesAndChains(t *testing.T) {
	orig := audioFixture(t, "opus.ogg")
	broken := bytes.Clone(orig)
	broken[len(broken)-5] ^= 0x40
	f := openBytes(t, broken, "broken")
	var err error
	for err == nil {
		_, err = f.ReadPacket()
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("a page whose checksum fails: %v, want ErrMalformed", err)
	}
	chained := append(bytes.Clone(orig), audioFixture(t, "vorbis.ogg")...)
	f = openBytes(t, chained, "chained")
	for err = nil; err == nil; {
		_, err = f.ReadPacket()
	}
	if !errors.Is(err, errChained) {
		t.Errorf("a chained file: %v, want errChained", err)
	}
	if _, err := Open(bytes.NewReader(orig[:200]), 200, "short"); !errors.Is(err, ErrMalformed) {
		t.Errorf("a file cut in its headers: %v, want ErrMalformed", err)
	}
}

// The Vorbis modes found back from the end of the setup header are the
// ones the header codes.
func TestVorbisModesFromTheSetupHeader(t *testing.T) {
	f := openBytes(t, audioFixture(t, "vorbis.ogg"), "vorbis.ogg")
	_, _, setup, err := XiphHeaders(f.Tracks()[0].Config)
	if err != nil {
		t.Fatal(err)
	}
	m := vorbisModes{blocks: [2]int{512, 512}}
	if err := m.parse(setup); err != nil {
		t.Fatal(err)
	}
	for i, p := range readPackets(t, f) {
		if n, err := m.blockSize(p.Data); n != 512 || err != nil {
			t.Errorf("packet %d: block of %d: %v, want the stream's 512", i, n, err)
		}
	}
	if err := m.parse(setup[:40]); !errors.Is(err, ErrMalformed) {
		t.Errorf("a cut setup header: %v, want ErrMalformed", err)
	}
}

// An MP3 file's LAME tag names the encoder's delay and padding, which
// come off its length and, the delay, its timestamps; a file without
// one lasts its frames, counted on opening.
func TestMP3LengthAndDelay(t *testing.T) {
	const frame = 576 * time.Second / 8000
	lame := openBytes(t, audioFixture(t, "tone.mp3"), "tone.mp3")
	delay := (576 + mp3DecoderDelay) * time.Second / 8000
	if tr := lame.Tracks()[0]; tr.CodecDelay != delay || lame.Duration() != time.Second || tr.Channels != 1 {
		t.Errorf("track %+v lasting %v, want the LAME tag's %v delay and a second", tr, lame.Duration(), delay)
	}
	if p, err := lame.ReadPacket(); err != nil || p.PTS != -delay || p.Duration != frame {
		t.Errorf("first packet %v at %v lasting %v, want one frame from %v", err, p.PTS, p.Duration, -delay)
	}
	cbr := audioFixture(t, "cbr.mp3")
	tagged := append(bytes.Clone(cbr), []byte("APETAGEX\xd0\x07\x00\x00\x20\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00")...)
	tagged = append(tagged, append([]byte("TAG"), make([]byte, id3v1Size-3)...)...)
	for name, b := range map[string][]byte{"cbr.mp3": cbr, "with APEv2 and ID3v1 tags": tagged} {
		f := openBytes(t, b, name)
		if f.Duration() != 16*frame || f.Tracks()[0].CodecDelay != 0 || f.Cover() != nil {
			t.Errorf("%s: %v of delay %v, want the 16 frames of %v undelayed", name, f.Duration(), f.Tracks()[0].CodecDelay, frame)
		}
		if got := readPackets(t, f); len(got) != 16 || got[15].PTS != 15*frame {
			t.Errorf("%s: %d packets", name, len(got))
		}
	}
	cut := openBytes(t, cbr[:300], "cut")
	var err error
	for n := 0; err == nil; n++ {
		if _, err = cut.ReadPacket(); err == nil && n == 2 {
			t.Fatal("a stream cut after its second frame reads a third")
		}
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("a stream cut in its third frame: %v, want ErrMalformed", err)
	}
	junk := append([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"), bytes.Repeat([]byte{0x55}, 64)...)
	if _, err := Open(bytes.NewReader(junk), int64(len(junk)), "junk"); !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("an ID3v2 tag and no frame: %v, want ErrUnknownFormat", err)
	}
}
