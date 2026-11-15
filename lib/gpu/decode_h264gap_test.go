package gpu

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys-lib/internal/nal"
	"github.com/stubbedev/lys-lib/media/demux"
)

// annexBUnits splits an Annex B byte stream into its NAL units.
func annexBUnits(b []byte) [][]byte {
	var units [][]byte
	start := -1
	for i := 0; i+2 < len(b); i++ {
		if b[i] != 0 || b[i+1] != 0 || b[i+2] != 1 {
			continue
		}
		if start >= 0 {
			units = append(units, bytes.TrimRight(b[start:i], "\x00"))
		}
		start = i + 3
		i += 2
	}
	if start >= 0 {
		units = append(units, b[start:])
	}
	return units
}

// allowGaps is sps, an SPS NAL unit of pic_order_cnt_type 0 or 2 and,
// of a High profile, of no scaling matrix, with
// gaps_in_frame_num_value_allowed_flag set.
func allowGaps(t *testing.T, sps []byte) []byte {
	t.Helper()
	rbsp := nal.Unescape(nil, sps[1:])
	r := nal.NewReader(rbsp)
	profile := r.U(8)
	r.Skip(16)
	r.UE()
	switch profile {
	case h264Baseline, h264Main, 88:
	case 100:
		if chroma := r.UE(); chroma == 3 {
			t.Fatal("SPS of 4:4:4")
		}
		r.UE()
		r.UE()
		r.Skip(1)
		if r.Flag() {
			t.Fatal("SPS with a scaling matrix")
		}
	default:
		t.Fatalf("SPS of profile_idc %d", profile)
	}
	r.UE()
	switch typ := r.UE(); typ {
	case 0:
		r.UE()
	case 2:
	default:
		t.Fatalf("SPS of pic_order_cnt_type %d, want 0 or 2", typ)
	}
	r.UE()
	at := r.Pos()
	rbsp[at/8] |= 0x80 >> (at % 8)
	var w nal.Writer
	for _, b := range rbsp {
		w.U(8, uint32(b))
	}
	return w.Unit(sps[0])
}

// A stream whose SPS allows gaps in frame_num, with reference frames
// missing, decodes bit for bit as ffmpeg decodes it: the frames the
// gaps leave non-existing are inferred, in the reference lists where
// the standard puts them, each standing in with the reference frame
// before it, which is how ffmpeg conceals them too. The frames after
// a gap predict from the frames missing; the stream is coded without
// the deblocking filter, which ffmpeg runs as if each inferred frame
// were the frame it copies.
func TestDecodeH264FrameNumGap(t *testing.T) {
	t.Parallel()
	d := decodingDevice(t, demux.H264)
	dir := t.TempDir()
	src := filepath.Join(dir, "a.h264")
	runFFmpeg(t, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x96:rate=25:duration=2", "-pix_fmt", "yuv420p",
		"-c:v", "libx264", "-profile:v", "baseline", "-x264-params", "keyint=100:ref=3:no-deblock=1", src)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	drop := map[int]bool{10: true, 20: true, 21: true, 22: true}
	var units [][]byte
	pictures := 0
	for _, u := range annexBUnits(b) {
		switch nal.H264Type(u) {
		case nal.H264NALSPS:
			u = allowGaps(t, u)
		case nal.H264NALSlice, nal.H264NALIDR:
			pictures++
			if drop[pictures-1] {
				continue
			}
		}
		units = append(units, u)
	}
	gapped := filepath.Join(dir, "gapped.h264")
	if err := os.WriteFile(gapped, nal.AnnexB(units...), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "gapped.mp4")
	runFFmpeg(t, "-v", "error", "-y", "-framerate", "25", "-i", gapped, "-c", "copy", path)
	cl := ffmpegDecoded(t, path, 160, 96, nv12Samples)
	if want := pictures - len(drop); len(cl.frames) != want {
		t.Fatalf("ffmpeg decodes %d frames, want the %d left", len(cl.frames), want)
	}
	track, pkts := packets(t, path)
	v, err := d.NewVideoDecoder(track)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	checkFrames(t, cl, decodeAll(t, v, pkts), 0)
}

// skip leaves n frames out before the next frame cut in, as an
// encoder whose frames are lost: frame_num and picture order count go
// on past them.
func (c *h264Cut) skip(n uint32) {
	c.prevRef += n
	c.frames += int32(n)
}

// gappedCut is a stream cut of a source's pictures, frame_num skipping
// gaps[k] frames before the k-th of its frames (a frame, or a frame's
// two fields).
type gappedCut func(t *testing.T) (cut *h264Cut, gaps map[int]uint32)

// Sequences of fields and of macroblock-adaptive frames whose SPS
// allows gaps in frame_num, frames missing, decode bit for bit as
// ffmpeg decodes them: each frame a gap leaves non-existing is inferred
// as a frame picture copying the latest reference frame, both its
// fields references. ffmpeg marks only the field of the picture after
// the gap a reference in each such frame, where 8.2.5.2 marks both, so
// the field streams keep one reference frame and one reference field
// in their lists, where both orders agree; frames, whose lists hold
// frames, keep three.
func TestDecodeH264FrameNumGapInFields(t *testing.T) {
	t.Parallel()
	pairs := func(first h264Fields, gaps map[int]uint32) gappedCut {
		return func(t *testing.T) (*h264Cut, map[int]uint32) {
			src := pFields(t, 1.2, 1)
			cut := newH264Cut(t, fieldSPS(t, src.sps, true), src.pps)
			for i := 0; i+1 < len(src.pics); i += 2 {
				cut.skip(gaps[i/2])
				cut.field(src, i, first, false)
				cut.field(src, i+1, h264Frame&^first, true)
			}
			return cut, gaps
		}
	}
	frames := func(sps func(t *testing.T, src *h264Source) []byte, args ...string) gappedCut {
		return func(t *testing.T) (*h264Cut, map[int]uint32) {
			src := x264Source(t, fieldsW, fieldsH, 1, append([]string{"-bf", "0", "-refs", "3"}, args...)...)
			cut := newH264Cut(t, sps(t, src), src.pps)
			if cut.seq.FrameMbsOnly {
				t.Fatal("the SPS codes frames alone")
			}
			gaps := map[int]uint32{6: 1, 10: 2}
			for i := range src.pics {
				cut.skip(gaps[i])
				cut.frame(src, i)
			}
			return cut, gaps
		}
	}
	for _, c := range []struct {
		name string
		cut  gappedCut
	}{
		{"field pairs, top first, a frame missing", pairs(h264Top, map[int]uint32{5: 1})},
		{"field pairs, bottom first, three frames missing", pairs(h264Bottom, map[int]uint32{5: 3})},
		{"frames in a sequence of fields", frames(func(t *testing.T, _ *h264Source) []byte {
			return fieldSPS(t, x264Source(t, fieldsW, fieldH, 0.2, "-bf", "0", "-refs", "3", "-x264-params", "keyint=250:no-deblock=1").sps, true)
		}, "-x264-params", "keyint=250:no-deblock=1")},
		{"macroblock-adaptive frames", frames(func(t *testing.T, src *h264Source) []byte { return allowGaps(t, src.sps) }, "-vf", "interlace", "-flags", "+ilme+ildct", "-x264-params", "keyint=250:no-deblock=1")},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := decodingDevice(t, demux.H264)
			cut, gaps := c.cut(t)
			if !cut.seq.GapsInFrameNumAllowed || len(gaps) == 0 {
				t.Fatal("the SPS allows no gaps, or the cut skips no frame_num")
			}
			want := cut.reference(fieldsW, fieldsH)
			v, err := d.NewVideoDecoder(cut.track())
			if err != nil {
				t.Fatal(err)
			}
			defer v.Close()
			checkFrames(t, want, decodeAll(t, v, cut.packets(false)), 0)
		})
	}
}
