package media

import (
	"errors"
	"image/color"
	"io"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stubbedev/lys/lib/pulse/native"

	"github.com/stubbedev/lys/ui/anim"
)

// coverBlue is the cover's colour, 0x3060a0 as FFmpeg's lavfi rendered
// it.
var coverBlue = color.RGBA{0x2e, 0x5f, 0x9e, 0xff}

var soundFixtures = []struct {
	name   string
	format AudioFormat
	cover  bool
	// onsetSlack is as fixtures' is; lossless FLAC is off by nothing
	// but the two 8 kHz samples the tone takes to pass onsetLevel.
	onsetSlack time.Duration
}{
	{"tone.m4a", AudioFormat{Rate: 44100, Layout: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}}, true, 6 * time.Millisecond},
	{"tone.mka", AudioFormat{Rate: 48000, Layout: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}}, false, 3 * time.Millisecond},
	{"tone.ogg", AudioFormat{Rate: 22050, Layout: native.ChannelMap{native.ChannelMono}}, false, 3 * time.Millisecond},
	{"tone.flac", AudioFormat{Rate: 8000, Layout: native.ChannelMap{native.ChannelMono}}, true, 2 * time.Second / 8000},
}

func TestASoundFileKeepsTime(t *testing.T) {
	for _, c := range soundFixtures {
		t.Run(c.name, func(t *testing.T) { checkSoundTime(t, openFixture(t, c.name).(Audible), c.onsetSlack) })
	}
}

// A file of sound alone shows one picture, its cover or the
// placeholder, for the whole of its sound, and shows it again after a
// seek.
func TestASoundFileShowsItsCover(t *testing.T) {
	for _, c := range soundFixtures {
		t.Run(c.name, func(t *testing.T) {
			src := openFixture(t, c.name)
			if _, ok := src.(TextureSource); ok {
				t.Fatal("a file without video decodes on the GPU")
			}
			stills, ok := src.(StillSource)
			if !ok {
				t.Fatal("the picture is not a still")
			}
			a, ok := src.(Audible)
			if !ok {
				t.Fatal("a sound file is silent")
			}
			if f := a.AudioFormat(); f.Rate != c.format.Rate || len(f.Layout) != len(c.format.Layout) || f.Layout[0] != c.format.Layout[0] {
				t.Errorf("audio format %+v, want %+v", f, c.format)
			}
			info := src.Info()
			if !within(info.Duration, fixtureDuration, 10*time.Millisecond) {
				t.Errorf("duration %v, want a second", info.Duration)
			}
			for range 2 {
				still, pts, err := stills.ReadStill()
				if err != nil || pts != 0 {
					t.Fatalf("picture at %v: %v", pts, err)
				}
				if c.cover && (info.Width != 16 || info.Height != 16 || still.RGBAAt(8, 8) != coverBlue) {
					t.Errorf("a %dx%d picture, its middle %v: want the 16x16 cover in %v", info.Width, info.Height, still.RGBAAt(8, 8), coverBlue)
				}
				if !c.cover && (info.Width != placeholderSide || still.RGBAAt(0, 0) == still.RGBAAt(placeholderSide*9/16, placeholderSide/2)) {
					t.Errorf("a %dx%d picture without the placeholder's quaver", info.Width, info.Height)
				}
				if _, _, err := stills.ReadStill(); !errors.Is(err, io.EOF) {
					t.Fatalf("a second picture: %v", err)
				}
				if err := src.(Seeker).Seek(600 * time.Millisecond); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// sinkOut is an output that takes the whole of the PCM when it opens,
// as a sound server's buffer would over time.
type sinkOut struct {
	fakeOut
	frames int
}

type sinks struct{ all []*sinkOut }

func (s *sinks) open(pcm io.Reader, f AudioFormat, _ string) (audioOut, error) {
	b, err := io.ReadAll(pcm)
	if err != nil {
		return nil, err
	}
	out := &sinkOut{frames: len(b) / f.frameBytes()}
	s.all = append(s.all, out)
	return out, nil
}

// A file of sound alone plays through to its end on the sound's clock,
// its picture up from the start.
func TestASoundFilePlaysToItsEndOnTheAudioClock(t *testing.T) {
	for _, c := range soundFixtures {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				src, err := OpenFile(fixture(t, c.name))
				if err != nil {
					t.Fatal(err)
				}
				o := &sinks{}
				s := newStream(new(anim.Clock), src, "test", o.open)
				step := func(d time.Duration) {
					time.Sleep(d)
					synctest.Wait()
					s.advance(time.Now())
				}
				step(0)
				if s.Frame().RGBA == nil || s.DecodesOnGPU() || !s.HasAudio() || !s.Seekable() {
					t.Fatal("the stream shows no picture, decodes on the GPU, is silent or cannot seek")
				}
				s.Play()
				synctest.Wait()
				if len(o.all) != 1 {
					t.Fatalf("%d outputs opened", len(o.all))
				}
				out := o.all[0]
				if got := samplesAt(out.frames, c.format.Rate); !within(got, fixtureDuration, 50*time.Millisecond) {
					t.Errorf("the sink took %v of sound, want the second", got)
				}
				for _, played := range []time.Duration{100 * time.Millisecond, 450 * time.Millisecond, 900 * time.Millisecond} {
					out.played = played
					step(5 * time.Second)
					if s.Position() != played || s.Ended() {
						t.Fatalf("the sink played %v, the stream is at %v (ended %v)", played, s.Position(), s.Ended())
					}
				}
				out.played, out.drained = samplesAt(out.frames, c.format.Rate)-audioLatency, true
				step(0)
				for range 10 {
					step(20 * time.Millisecond)
				}
				if !s.Ended() || s.Playing() || s.Position() != src.Info().Duration || !out.closed {
					t.Errorf("past the end: ended %v, playing %v at %v of %v, output closed %v", s.Ended(), s.Playing(), s.Position(), src.Info().Duration, out.closed)
				}
				if s.Frame().RGBA == nil {
					t.Error("the picture went away")
				}
				s.Close()
				if err := s.Wait(); err != nil {
					t.Error(err)
				}
			})
		})
	}
}

func TestOpenFilePlaysVideoWithFLACSound(t *testing.T) {
	src := openFixture(t, "h264-flac.mkv")
	if _, ok := src.(TextureSource); !ok {
		t.Fatal("the video does not decode on the GPU")
	}
	a, ok := src.(Audible)
	if !ok {
		t.Fatal("the FLAC sound is not heard")
	}
	f := a.AudioFormat()
	if f.Rate != 8000 || len(f.Layout) != 1 {
		t.Errorf("audio format %+v, want 8 kHz mono", f)
	}
	pcm := readPCM(t, a, 0)
	if len(pcm) != 8000 || slices.ContainsFunc(pcm, func(v int16) bool { return v != 0 }) {
		t.Errorf("%d samples, want the fixture's second of silence", len(pcm))
	}
}

func TestPlaceholderIsNeutral(t *testing.T) {
	img := placeholder(placeholderSide)
	for i := 0; i < len(img.Pix); i += 4 {
		if r, g, b := img.Pix[i], img.Pix[i+1], img.Pix[i+2]; max(r, g, b)-min(r, g, b) > 8 || img.Pix[i+3] != 0xff {
			t.Fatalf("pixel %d is %v, want an opaque grey", i/4, img.Pix[i:i+4])
		}
	}
}
