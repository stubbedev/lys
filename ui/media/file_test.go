package media

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/media/demux"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/testbuild"
)

// The fixtures are one second of a 192x144 picture, red until 500 ms and
// green from there, with a 440 Hz tone over silence from 500 ms on:
//
//	ffmpeg -f lavfi -i 'color=c=red:s=192x144:r=25:d=1,drawbox=enable=gte(t\,0.5):c=green:t=fill' \
//	       -f lavfi -i 'aevalsrc=if(gte(t\,0.5)\,0.5*sin(2*PI*440*t)\,0):s=44100:c=stereo:d=1' \
//	       -c:v libx264 -g 10 -bf 2 -c:a aac h264-aac.mp4
//
// h265-opus.mkv is the same in H.265 (keyint 10, two B-frames) and Opus
// at 48 kHz, h264-flac.mkv has a second of 8 kHz FLAC silence, vp9.webm
// VP9 video and tone.mka the Opus tone alone. The other files of sound
// alone carry the tone too: tone.m4a in AAC with a 16x16 PNG cover,
// tone.ogg in 22.05 kHz mono Vorbis and tone.flac in 8 kHz mono FLAC
// with the same cover:
//
//	ffmpeg -f lavfi -i 'color=c=0x3060a0:s=16x16:d=1' -frames:v 1 cover.png
//	ffmpeg -f lavfi -i 'aevalsrc=…:s=44100:c=stereo:d=1' -i cover.png -map 0 -map 1 -c:a aac -b:a 32k \
//	       -c:v png -disposition:v attached_pic tone.m4a
//
// with FFmpeg's PADDING block taken out of tone.flac. tone.mp3 is
// lib/media/demux/testdata's, 8 kHz mono MP3 with the cover, its quiet
// 220 Hz tone turning into the 440 Hz one at 500 ms; tone-mp3.mka is
// its frames copied into Matroska, and tone-mp3.mp4 the stereo tone in
// 44.1 kHz MP3:
//
//	ffmpeg -i tone.mp3 -map 0:a -c:a copy tone-mp3.mka
//	ffmpeg -f lavfi -i 'aevalsrc=…:s=44100:c=stereo:d=1' -c:a libmp3lame -b:a 64k tone-mp3.mp4
//
// ac3.mka is 200 ms of AC-3 and subtitles.mkv a subtitle track alone.
const (
	fixtureDuration = time.Second
	fixtureOnset    = 500 * time.Millisecond
	onsetLevel      = 8000
)

var fixtures = []struct {
	name   string
	format AudioFormat
	// onsetSlack is how far the tone's decoded onset may stray: the
	// codec smears it by a fraction of its frame, far less than the
	// encoder delay a wrong start would shift it by.
	onsetSlack time.Duration
}{
	{"h264-aac.mp4", AudioFormat{Rate: 44100, Layout: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}}, 6 * time.Millisecond},
	{"h265-opus.mkv", AudioFormat{Rate: 48000, Layout: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}}, 3 * time.Millisecond},
}

func fixture(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func openFixture(t testing.TB, name string) Source {
	t.Helper()
	src, err := OpenFile(fixture(t, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := src.Close(); err != nil {
			t.Error(err)
		}
	})
	return src
}

func readPCM(t *testing.T, a Audible, from time.Duration) []int16 {
	t.Helper()
	pcm, err := a.OpenAudio(from)
	if err != nil {
		t.Fatal(err)
	}
	defer pcm.Close()
	b, err := io.ReadAll(pcm)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]int16, len(b)/2)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(b[2*i:]))
	}
	return out
}

// onset is when the first sample past onsetLevel plays, frames counted
// from the PCM's start.
func onset(pcm []int16, f AudioFormat) time.Duration {
	for i, v := range pcm {
		if v > onsetLevel || v < -onsetLevel {
			return samplesAt(i/len(f.Layout), f.Rate)
		}
	}
	return -1
}

func within(got, want, slack time.Duration) bool { return got >= want-slack && got <= want+slack }

func TestOpenFileFindsTheTracks(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			src := openFixture(t, c.name)
			if i := src.Info(); i.Width != 192 || i.Height != 144 || !within(i.Duration, fixtureDuration, 50*time.Millisecond) {
				t.Errorf("info %+v", i)
			}
			if _, ok := src.(TextureSource); !ok {
				t.Error("the file's video does not decode on the GPU")
			}
			a, ok := src.(Audible)
			if !ok {
				t.Fatal("a file with sound is not Audible")
			}
			if f := a.AudioFormat(); f.Rate != c.format.Rate || len(f.Layout) != 2 || f.Layout[0] != c.format.Layout[0] || f.Layout[1] != c.format.Layout[1] {
				t.Errorf("audio format %+v, want %+v", f, c.format)
			}
		})
	}
}

// The sound starts at its timestamps: the AAC encoder's priming the MP4
// edit list names, and the Opus pre-skip, are dropped, so the tone
// sounds 500 ms in as the picture turns green, and seeking lands the
// PCM's first sample on the target.
func TestOpenFileSoundKeepsTime(t *testing.T) {
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) { checkSoundTime(t, openFixture(t, c.name).(Audible), c.onsetSlack) })
	}
}

// checkSoundTime checks that the tone of a fixture sounds at 500 ms
// within slack whatever the PCM starts from, and that the PCM runs to
// the fixture's end.
func checkSoundTime(t *testing.T, a Audible, slack time.Duration) {
	t.Helper()
	f := a.AudioFormat()
	for _, from := range []time.Duration{0, 250 * time.Millisecond, 0, 400 * time.Millisecond} {
		pcm := readPCM(t, a, from)
		if got := onset(pcm, f); !within(got, fixtureOnset-from, slack) {
			t.Errorf("from %v: the tone sounds %v in, want %v", from, got, fixtureOnset-from)
		}
		length := samplesAt(len(pcm)/len(f.Layout), f.Rate)
		if length < fixtureDuration-from || length > fixtureDuration-from+50*time.Millisecond {
			t.Errorf("from %v: %v of sound, want the %v to the end", from, length, fixtureDuration-from)
		}
	}
	pcm := readPCM(t, a, 750*time.Millisecond)
	if got := onset(pcm, f); got < 0 || got > 5*time.Millisecond {
		t.Errorf("from 750 ms the tone sounds %v in, want at once", got)
	}
}

func TestOpeningTheSoundAgainClosesTheReaderBefore(t *testing.T) {
	a := openFixture(t, "h265-opus.mkv").(Audible)
	first, err := a.OpenAudio(0)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.OpenAudio(0)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.Read(make([]byte, 64)); !errors.Is(err, errClosed) {
		t.Errorf("the reader before reads on: %v", err)
	}
	if n, err := second.Read(make([]byte, 64)); n != 64 || err != nil {
		t.Errorf("the new reader read %d bytes: %v", n, err)
	}
}

func TestSteadySoundAllocatesNothing(t *testing.T) {
	if !testbuild.AllocBudgetsHold {
		t.Skip("allocation budgets do not hold in this build")
	}
	for _, c := range fixtures {
		t.Run(c.name, func(t *testing.T) {
			a := openFixture(t, c.name).(Audible)
			pcm, err := a.OpenAudio(0)
			if err != nil {
				t.Fatal(err)
			}
			defer pcm.Close()
			buf := make([]byte, c.format.Rate/audioChunksPerSec*c.format.frameBytes())
			if _, err := io.ReadFull(pcm, buf); err != nil {
				t.Fatal(err)
			}
			if allocs := testing.AllocsPerRun(20, func() {
				if _, err := io.ReadFull(pcm, buf); err != nil {
					t.Fatal(err)
				}
			}); allocs != 0 {
				t.Errorf("%v allocations per 10 ms of sound, want none", allocs)
			}
		})
	}
}

func TestOpenFileSaysWhatItCannotPlay(t *testing.T) {
	_, err := OpenFile(fixture(t, "ac3.mka"))
	var unsupported *demux.UnsupportedCodecError
	if !errors.As(err, &unsupported) || !strings.Contains(unsupported.Codec, "A_AC3") || unsupported.File != fixture(t, "ac3.mka") {
		t.Errorf("AC-3 sound: %v", err)
	}
	if _, err := OpenFile(fixture(t, "subtitles.mkv")); err == nil || !strings.Contains(err.Error(), "neither video nor sound") || !strings.Contains(err.Error(), "subtitles.mkv") {
		t.Errorf("a file of subtitles: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "absent.mkv")
	if _, err := OpenFile(absent); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), absent) {
		t.Errorf("a missing file: %v", err)
	}
	if _, err := OpenFile(fixture(t, "../media.go")); !errors.Is(err, demux.ErrUnknownFormat) {
		t.Errorf("a Go file: %v", err)
	}
}

func TestWithoutAudioKeepsTheFileOnTheGPU(t *testing.T) {
	src := WithoutAudio(openFixture(t, "h265-opus.mkv"))
	if _, ok := src.(Audible); ok {
		t.Error("WithoutAudio kept the sound")
	}
	if _, ok := src.(TextureSource); !ok {
		t.Error("WithoutAudio lost the GPU decoding")
	}
}

// OpenFile opens a YUV4MPEG2 file by its magic, whatever its name, as a
// seekable stream of its frames that closes the file; a file that is
// neither that nor a container is an error naming it.
func TestOpenFileOpensAYUV4MPEG2File(t *testing.T) {
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.bin")
	raw := y4mBytes("YUV4MPEG2 W2 H2 F10:1 XCOLORRANGE=FULL", 2, 2, 1, 1, false, yuv{255, 128, 128}, yuv{0, 128, 128}, yuv{255, 128, 128})
	if err := os.WriteFile(clip, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	src, err := OpenFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	frames, ok := src.(FrameSource)
	if !ok {
		t.Fatalf("the clip opens as %T, want a FrameSource", src)
	}
	if _, ok := src.(Seeker); !ok {
		t.Fatalf("the clip opens as %T, want it seekable", src)
	}
	if info := frames.Info(); info.Width != 2 || info.Height != 2 || info.Duration != 300*time.Millisecond {
		t.Fatalf("the clip is %+v, want 2x2 for 300ms", info)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	short := filepath.Join(dir, "short.y4m")
	if err := os.WriteFile(short, []byte("YUV4"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFile(short); err == nil || !strings.Contains(err.Error(), short) {
		t.Fatalf("a file of four bytes opens with %v, want an error naming it", err)
	}
	if _, err := OpenFile(filepath.Join(dir, "missing.y4m")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing file opens with %v, want os.ErrNotExist", err)
	}
}
