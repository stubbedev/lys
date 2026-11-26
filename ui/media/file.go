package media

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/gpu"
	"github.com/stubbedev/lys/lib/media/demux"
)

// soundCodecs are the audio codecs OpenFile decodes.
var soundCodecs = []demux.Codec{demux.AAC, demux.Opus, demux.Vorbis, demux.FLAC, demux.MP3}

// OpenFile opens a media file demuxed in process: MP4, Matroska/WebM,
// Ogg, native FLAC or MPEG audio (MP3). Its first video track decodes on
// the GPU of the Video showing it (a TextureSource), and its first sound
// track in AAC-LC, Opus, Vorbis, FLAC or MP3 decodes in Go, so the source
// is Audible exactly when the file has sound. A file of sound alone
// plays with its cover art for a picture, a neutral placeholder when it
// embeds none. A sound track in any other codec is a
// demux.UnsupportedCodecError, and a file with neither video nor
// sound, or with a cover that does not decode, is an error too; a video
// codec the device does not decode fails when the Video first shows the
// stream. Seeking lands on the keyframe before the target and decodes
// from there.
func OpenFile(path string) (Source, error) {
	f, err := demux.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("media: open %s: %w", path, err)
	}
	src, err := newFile(path, f)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return src, nil
}

// soundTrack is the index of f's first sound track in a codec OpenFile
// decodes, -1 when it has no sound; a file whose sound is all in other
// codecs is an error.
func soundTrack(f *demux.File) (int, error) {
	tracks := f.Tracks()
	if ai := slices.IndexFunc(tracks, func(t demux.Track) bool { return t.Kind == demux.Audio && slices.Contains(soundCodecs, t.Codec) }); ai >= 0 {
		return ai, nil
	}
	if other := slices.IndexFunc(tracks, func(t demux.Track) bool { return t.Kind == demux.Audio }); other >= 0 {
		return -1, fmt.Errorf("media: %w", f.Unsupported(other))
	}
	return -1, nil
}

// newFile makes the source of the file f, which the source owns once
// it is made.
func newFile(path string, f *demux.File) (Source, error) {
	tracks := f.Tracks()
	ai, err := soundTrack(f)
	if err != nil {
		return nil, err
	}
	duration := f.Duration()
	vi := slices.IndexFunc(tracks, func(t demux.Track) bool { return t.Kind == demux.Video })
	if vi < 0 {
		if ai < 0 {
			return nil, fmt.Errorf("media: %s has neither video nor sound", path)
		}
		snd, err := openSound(path, f, tracks[ai])
		if err != nil {
			return nil, err
		}
		if duration == 0 {
			duration = tracks[ai].Duration
		}
		return newSoundFile(path, snd, duration, f.Cover())
	}
	if err := f.Select(vi); err != nil {
		return nil, fmt.Errorf("media: %w", err)
	}
	video := tracks[vi]
	if duration == 0 {
		duration = video.Duration
	}
	src := &file{
		path: path, demux: f, video: video, unsupported: f.Unsupported(vi),
		info: Info{Width: video.Width, Height: video.Height, Duration: duration},
	}
	if ai < 0 {
		return src, nil
	}
	audio, err := demux.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("media: open the sound of %s: %w", path, err)
	}
	snd, err := openSound(path, audio, tracks[ai])
	if err != nil {
		return nil, errors.Join(err, audio.Close())
	}
	return &audibleFile{file: src, sound: snd}, nil
}

// file is a media file's video, its packets read by the Stream's packet
// goroutine and decoded on the GPU.
type file struct {
	path        string
	demux       *demux.File
	video       demux.Track
	unsupported error
	info        Info
}

func (f *file) Info() Info { return f.info }

func (f *file) NewVideoDecoder(dev *gpu.Device) (VideoDecoder, error) {
	dec, err := dev.NewVideoDecoder(f.video)
	switch {
	case errors.Is(err, gpu.ErrNoVideoDecoder):
		return nil, fmt.Errorf("media: %w: %w", f.unsupported, err)
	case err != nil:
		return nil, fmt.Errorf("media: %s: %w", f.path, err)
	}
	return dec, nil
}

func (f *file) ReadPacket() (demux.Packet, error) {
	p, err := f.demux.ReadPacket()
	if err != nil && !errors.Is(err, io.EOF) {
		return demux.Packet{}, fmt.Errorf("media: %w", err)
	}
	return p, err
}

func (f *file) Seek(t time.Duration) error {
	if _, err := f.demux.Seek(t); err != nil {
		return fmt.Errorf("media: %w", err)
	}
	return nil
}

func (f *file) Close() error { return f.demux.Close() }

type audibleFile struct {
	*file
	sound *sound
}

func (f *audibleFile) AudioFormat() AudioFormat { return f.sound.codec.format }

func (f *audibleFile) OpenAudio(t time.Duration) (io.ReadCloser, error) { return f.sound.open(t) }

func (f *audibleFile) Close() error { return errors.Join(f.file.Close(), f.sound.close()) }
