package vorbis

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/media/demux"
)

// The demuxer times an Ogg stream's packets from the modes it finds at
// the end of the setup header: each lasts what the decoder outputs for
// it, and they add up to the stream Decode decodes.
func TestDemuxedOggPacketsLastWhatTheyDecodeTo(t *testing.T) {
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			data := readFixture(t, f.name+".ogg")
			file, err := demux.Open(bytes.NewReader(data), int64(len(data)), f.name)
			if err != nil {
				t.Fatal(err)
			}
			track := file.Tracks()[0]
			id, comment, setup, err := demux.XiphHeaders(track.Config)
			if err != nil {
				t.Fatal(err)
			}
			d, err := NewDecoder(id, comment, setup)
			if err != nil {
				t.Fatal(err)
			}
			var pcm []int16
			decoded := 0
			for i := 0; ; i++ {
				p, err := file.ReadPacket()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				before := len(pcm)
				if pcm, err = d.Decode(pcm, p.Data); err != nil {
					t.Fatal(err)
				}
				frames := (len(pcm) - before) / f.channels
				if want := int(math.Round(p.Duration.Seconds() * float64(f.rate))); frames != want && !last(p, track.Duration) {
					t.Fatalf("packet %d decodes to %d frames, the demuxer times it %v (%d frames)", i, frames, p.Duration, want)
				}
				if at := time.Duration(decoded) * time.Second / time.Duration(f.rate); p.PTS-at > time.Millisecond || at-p.PTS > time.Millisecond {
					t.Fatalf("packet %d at %v, after %v of output", i, p.PTS, at)
				}
				decoded += frames
			}
			whole, err := Decode(data)
			if err != nil {
				t.Fatal(err)
			}
			if end := int(math.Round(track.Duration.Seconds() * float64(f.rate))); end != whole.Frames() {
				t.Errorf("the track lasts %v (%d frames), the stream decodes to %d", track.Duration, end, whole.Frames())
			}
		})
	}
}

// last reports a packet the stream's end cuts short.
func last(p demux.Packet, end time.Duration) bool { return p.PTS+p.Duration >= end-time.Millisecond }
