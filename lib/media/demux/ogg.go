package demux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/media/internal/flacstream"
	"github.com/stubbedev/lys/lib/media/internal/ogg"
	"github.com/stubbedev/lys/lib/media/internal/opustoc"
)

const (
	oggMagic       = "OggS"
	oggNoGranule   = -1
	oggMaxSegments = 255
	opusHeadSize   = 19
	oggFLACHead    = 13
	vorbisIDSize   = 30
)

// oggCodec is how a logical stream's packets are timed.
type oggCodec uint8

const (
	oggOther oggCodec = iota
	oggVorbis
	oggOpus
	oggFLAC
)

// oggStream is one logical stream of an Ogg file.
type oggStream struct {
	serial uint32
	codec  oggCodec
	info   Track
	rate   uint64
	// offset is the granule position of time zero: Opus's pre-skip.
	offset int64
	// headerPackets is how many packets lead the stream before its
	// data, skip how many of them are still to come.
	headerPackets, skip int
	headers             [][]byte
	modes               vorbisModes
	// prevBlock is the Vorbis block size of the packet before, zero
	// when unknown.
	prevBlock int
	// granule is the position at the end of the packet the previous
	// page completed, oggNoGranule when unknown.
	granule int64
	pending []byte
	// resync drops the packet a page continues from one this stream
	// did not read (the stream was not selected, or a seek skipped
	// it).
	resync bool
	// skipThrough is the offset of the last page whose packets a seek
	// drops; pages from there on play.
	skipThrough int64
}

// oggPacket is a packet of the page read last, its data in the file's
// arena.
type oggPacket struct {
	track      int
	start, end int
	from, to   int64
}

// oggPage is a page of the index a seek searches.
type oggPage struct {
	off     int64
	serial  uint32
	granule int64
}

type oggFile struct {
	src     *source
	streams []*oggStream
	infos   []Track
	dur     time.Duration
	pos     int64
	queue   []oggPacket
	arena   []byte
	index   []oggPage
	cover   cover
	// page is the page read last: its header fields and segments.
	flags   byte
	granule int64
	serial  uint32
	lacing  []byte
	body    []byte
	pageOff int64
}

var errChained = errors.New("chained Ogg streams are not supported")

func openOgg(src *source) (*oggFile, error) {
	o := &oggFile{src: src}
	if err := o.readHeaders(); err != nil {
		return nil, err
	}
	o.infos = make([]Track, len(o.streams))
	for i, s := range o.streams {
		last, err := o.lastGranule(s.serial)
		if err != nil {
			return nil, err
		}
		if s.rate > 0 && last > s.offset {
			s.info.Duration = ticksToTime(last-s.offset, s.rate)
		}
		o.dur = max(o.dur, s.info.Duration)
		o.infos[i] = s.info
	}
	o.restart()
	return o, nil
}

// readPage reads the page at off into the file's page fields,
// checking its checksum, and returns the offset after it.
func (o *oggFile) readPage(off int64) (int64, error) {
	h, err := o.src.view(off, ogg.HeaderSize)
	if err != nil {
		return 0, fmt.Errorf("the Ogg page at offset %d: %w", off, err)
	}
	if string(h[:4]) != oggMagic || h[4] != 0 {
		return 0, fmt.Errorf("no Ogg page at offset %d: %w", off, ErrMalformed)
	}
	nseg := int(h[26])
	seg, err := o.src.view(off+ogg.HeaderSize, nseg)
	if err != nil {
		return 0, fmt.Errorf("the Ogg page at offset %d: %w", off, err)
	}
	size := ogg.HeaderSize + nseg
	for _, l := range seg {
		size += int(l)
	}
	page, err := o.src.view(off, size)
	if err != nil {
		return 0, fmt.Errorf("the Ogg page at offset %d: %w", off, err)
	}
	if got, want := ogg.Checksum(page), binary.LittleEndian.Uint32(page[22:]); got != want {
		return 0, fmt.Errorf("the Ogg page at offset %d: checksum %08x, header says %08x: %w", off, got, want, ErrMalformed)
	}
	o.flags = page[5]
	o.granule = intconv.Int64Bits(binary.LittleEndian.Uint64(page[6:]))
	o.serial = binary.LittleEndian.Uint32(page[14:])
	o.lacing = page[ogg.HeaderSize : ogg.HeaderSize+nseg]
	o.body = page[ogg.HeaderSize+nseg:]
	o.pageOff = off
	return off + int64(size), nil
}

func (o *oggFile) stream(serial uint32) (int, *oggStream) {
	for i, s := range o.streams {
		if s.serial == serial {
			return i, s
		}
	}
	return -1, nil
}

// eachPacket calls fn with each packet of stream s the page read last
// completes, joining those that span pages in s.pending, and keeps the
// packet it leaves unfinished.
func (o *oggFile) eachPacket(s *oggStream, fn func(p []byte) error) error {
	continued := o.flags&ogg.Continued != 0
	switch {
	case continued && len(s.pending) == 0:
		s.resync = true
	case !continued && len(s.pending) > 0:
		return fmt.Errorf("the Ogg page at offset %d does not continue the packet before: %w", o.pageOff, ErrMalformed)
	}
	at := 0
	for i := 0; i < len(o.lacing); {
		start := at
		for i < len(o.lacing) {
			l := int(o.lacing[i])
			at += l
			i++
			if l < oggMaxSegments {
				break
			}
		}
		chunk := o.body[start:at]
		if i == len(o.lacing) && o.lacing[i-1] == oggMaxSegments {
			if !s.resync {
				s.pending = append(s.pending, chunk...)
			}
			return nil
		}
		p := chunk
		if len(s.pending) > 0 {
			s.pending = append(s.pending, chunk...)
			p = s.pending
		}
		if s.resync {
			s.resync = false
			continue
		}
		if err := fn(p); err != nil {
			return err
		}
		s.pending = s.pending[:0]
	}
	return nil
}

// readHeaders reads the pages that begin the streams, which come
// first, and the pages after them up to every stream's last header
// packet.
func (o *oggFile) readHeaders() error {
	beginning := true
	for off := int64(0); ; {
		if off >= o.src.size {
			return fmt.Errorf("the Ogg file ends before its streams' headers: %w", ErrMalformed)
		}
		next, err := o.readPage(off)
		if err != nil {
			return err
		}
		_, s := o.stream(o.serial)
		switch bos := o.flags&ogg.BOS != 0; {
		case bos && s == nil && !beginning:
			return fmt.Errorf("the Ogg stream %08x begins at offset %d after the others' data: %w", o.serial, off, errChained)
		case bos && s == nil:
			s = &oggStream{serial: o.serial, granule: oggNoGranule}
			s.info.Index = len(o.streams)
			o.streams = append(o.streams, s)
		case s == nil:
			return fmt.Errorf("the Ogg page at offset %d of stream %08x, which never began: %w", off, o.serial, ErrMalformed)
		case !bos:
			beginning = false
		}
		if err := o.eachPacket(s, func(p []byte) error { return o.header(s, p) }); err != nil {
			return err
		}
		off = next
		if !beginning && o.headersDone() {
			return nil
		}
	}
}

func (o *oggFile) headersDone() bool {
	for _, s := range o.streams {
		if s.codec != oggOther && (len(s.headers) == 0 || len(s.headers) < s.headerPackets) {
			return false
		}
	}
	return true
}

// header takes packet p of stream s while its headers are still to
// come.
func (o *oggFile) header(s *oggStream, p []byte) error {
	if len(s.headers) == 0 {
		if err := o.identify(s, p); err != nil {
			return fmt.Errorf("the Ogg stream %08x: %w", s.serial, err)
		}
	}
	if s.codec == oggOther || len(s.headers) >= s.headerPackets && len(s.headers) > 0 {
		return nil
	}
	if s.codec == oggFLAC && len(s.headers) > 0 && len(p) > 0 && p[0] == 0xff {
		s.headerPackets = len(s.headers)
		return nil
	}
	s.headers = append(s.headers, bytes.Clone(p))
	var err error
	switch {
	case s.codec == oggVorbis && len(s.headers) == 2:
		err = o.cover.offerComments(trimPrefix(p, "\x03vorbis"))
	case s.codec == oggOpus && len(s.headers) == 2:
		err = o.cover.offerComments(trimPrefix(p, "OpusTags"))
	case s.codec == oggFLAC && len(s.headers) > 1:
		err = flacstream.EachBlock(p, func(typ int, body []byte) error {
			switch typ {
			case flacstream.BlockPicture:
				return o.cover.offerFLAC(body)
			case flacstream.BlockVorbisComment:
				return o.cover.offerComments(body)
			}
			return nil
		})
	}
	if err != nil {
		return fmt.Errorf("the Ogg stream %08x header %d: %w", s.serial, len(s.headers)-1, err)
	}
	if s.codec == oggVorbis && len(s.headers) == 3 {
		return o.vorbisSetup(s)
	}
	return nil
}

func trimPrefix(p []byte, prefix string) []byte {
	b, _ := bytes.CutPrefix(p, []byte(prefix))
	return b
}

// identify reads the codec of stream s from its first packet p.
func (o *oggFile) identify(s *oggStream, p []byte) error {
	s.info.Kind, s.info.Codec = Other, Unknown
	switch {
	case bytes.HasPrefix(p, []byte("\x01vorbis")):
		if len(p) < vorbisIDSize {
			return fmt.Errorf("the Vorbis identification header of %d bytes: %w", len(p), ErrMalformed)
		}
		s.codec, s.headerPackets = oggVorbis, 3
		s.info.Kind, s.info.Codec, s.info.CodecName = Audio, Vorbis, "Vorbis"
		s.info.Channels = int(p[11])
		s.info.SampleRate = int(binary.LittleEndian.Uint32(p[12:]))
		s.modes.blocks = [2]int{1 << (p[28] & 15), 1 << (p[28] >> 4)}
	case bytes.HasPrefix(p, []byte("OpusHead")):
		if len(p) < opusHeadSize {
			return fmt.Errorf("OpusHead of %d bytes: %w", len(p), ErrMalformed)
		}
		s.codec, s.headerPackets = oggOpus, 2
		s.info.Kind, s.info.Codec, s.info.CodecName = Audio, Opus, "Opus"
		s.info.Config = bytes.Clone(p)
		s.info.Channels, s.info.SampleRate = int(p[9]), opusRate
		s.offset = int64(binary.LittleEndian.Uint16(p[10:]))
		s.info.CodecDelay = ticksToTime(s.offset, opusRate)
		s.info.SeekPreRoll = opusPreRoll
	case bytes.HasPrefix(p, []byte("\x7fFLAC")):
		if len(p) < oggFLACHead+flacstream.BlockHeaderSize || string(p[9:13]) != flacMagic {
			return fmt.Errorf("the Ogg FLAC header of %d bytes: %w", len(p), ErrMalformed)
		}
		var streamInfo []byte
		if err := flacstream.EachBlock(p[oggFLACHead:], func(typ int, body []byte) error {
			if typ == flacstream.BlockStreamInfo {
				streamInfo = bytes.Clone(body)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("%w: %w", ErrMalformed, err)
		}
		info, err := flacstream.ParseStreamInfo(streamInfo)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrMalformed, err)
		}
		s.codec, s.headerPackets = oggFLAC, int(binary.BigEndian.Uint16(p[7:]))+1
		if s.headerPackets == 1 {
			s.headerPackets = maxOggFLACHeaders
		}
		s.info.Kind, s.info.Codec, s.info.CodecName = Audio, FLAC, "FLAC"
		s.info.Config = streamInfo
		s.info.Channels, s.info.SampleRate = info.Channels, info.SampleRate
	case bytes.HasPrefix(p, []byte("\x80theora")):
		s.info.Kind, s.info.CodecName = Video, "Theora"
	case bytes.HasPrefix(p, []byte("OVP80")):
		s.info.Kind, s.info.CodecName = Video, "VP8 in Ogg"
	case bytes.HasPrefix(p, []byte("Speex   ")):
		s.info.Kind, s.info.CodecName = Audio, "Speex"
	case bytes.HasPrefix(p, []byte("fishead\x00")):
		s.info.CodecName = "Ogg Skeleton"
	default:
		s.info.CodecName = fmt.Sprintf("Ogg stream %q", p[:min(len(p), 8)])
	}
	s.rate = uint64(max(s.info.SampleRate, 0))
	return nil
}

// maxOggFLACHeaders stands for an unknown count of FLAC header
// packets: they run until the first frame.
const maxOggFLACHeaders = 1 << 16

func (o *oggFile) vorbisSetup(s *oggStream) error {
	if err := s.modes.parse(s.headers[2]); err != nil {
		return fmt.Errorf("the Ogg stream %08x: %w", s.serial, err)
	}
	id, comment, setup := s.headers[0], s.headers[1], s.headers[2]
	cfg := []byte{2}
	for _, h := range [][]byte{id, comment} {
		n := len(h)
		for ; n >= 0xff; n -= 0xff {
			cfg = append(cfg, 0xff)
		}
		cfg = append(cfg, intconv.Must[byte](n))
	}
	cfg = append(cfg, id...)
	cfg = append(cfg, comment...)
	s.info.Config = append(cfg, setup...)
	return nil
}

// lastGranule is the granule position of serial's last page.
func (o *oggFile) lastGranule(serial uint32) (int64, error) {
	const step = windowSize
	for end := o.src.size; end > 0; end -= step - ogg.HeaderSize {
		start := max(end-step, 0)
		b, err := o.src.view(start, int(end-start))
		if err != nil {
			return 0, err
		}
		for at := len(b) - ogg.HeaderSize; at >= 0; at-- {
			if b[at] != 'O' || string(b[at:at+4]) != oggMagic || binary.LittleEndian.Uint32(b[at+14:]) != serial {
				continue
			}
			if g := intconv.Int64Bits(binary.LittleEndian.Uint64(b[at+6:])); g >= 0 {
				return g, nil
			}
		}
		if start == 0 {
			break
		}
	}
	return 0, nil
}

// restart positions the file at its start, its header packets to be
// skipped again.
func (o *oggFile) restart() {
	o.pos, o.queue = 0, o.queue[:0]
	for _, s := range o.streams {
		s.skip, s.prevBlock, s.granule = len(s.headers), 0, oggNoGranule
		s.pending, s.resync, s.skipThrough = s.pending[:0], false, -1
	}
}

func (o *oggFile) tracks() []Track { return o.infos }

func (o *oggFile) duration() time.Duration { return o.dur }

func (o *oggFile) next(selected []bool, _ []byte) (Packet, error) {
	for len(o.queue) == 0 {
		if o.pos >= o.src.size {
			for i, s := range o.streams {
				if selected[i] && len(s.pending) > 0 && !s.resync {
					return Packet{}, fmt.Errorf("the Ogg stream %08x ends inside a packet: %w", s.serial, ErrMalformed)
				}
			}
			return Packet{}, io.EOF
		}
		if err := o.readData(selected); err != nil {
			return Packet{}, err
		}
	}
	q := o.queue[0]
	o.queue = o.queue[1:]
	s := o.streams[q.track]
	pts := ticksToTime(q.from-s.offset, s.rate)
	return Packet{
		Track: q.track, PTS: pts, Duration: ticksToTime(q.to-s.offset, s.rate) - pts,
		Key: true, Data: o.arena[q.start:q.end],
	}, nil
}

// readData reads the next page and queues the packets it completes of
// the selected streams, timed from the page's granule position.
func (o *oggFile) readData(selected []bool) error {
	next, err := o.readPage(o.pos)
	if err != nil {
		return err
	}
	o.pos = next
	i, s := o.stream(o.serial)
	switch {
	case s == nil && o.flags&ogg.BOS != 0:
		return fmt.Errorf("the Ogg stream %08x begins at offset %d: %w", o.serial, o.pageOff, errChained)
	case s == nil:
		return fmt.Errorf("the Ogg page at offset %d of stream %08x, which never began: %w", o.pageOff, o.serial, ErrMalformed)
	case !selected[i] || s.codec == oggOther:
		s.pending, s.resync = s.pending[:0], false
		return nil
	}
	o.queue, o.arena = o.queue[:0], o.arena[:0]
	skipping := o.pageOff <= s.skipThrough
	err = o.eachPacket(s, func(p []byte) error {
		if s.skip > 0 {
			s.skip--
			return nil
		}
		if skipping {
			return nil
		}
		dur, err := s.packetSamples(p)
		if err != nil {
			return fmt.Errorf("the Ogg stream %08x, page at offset %d: %w", s.serial, o.pageOff, err)
		}
		start := len(o.arena)
		o.arena = append(o.arena, p...)
		o.queue = append(o.queue, oggPacket{track: i, start: start, end: len(o.arena), to: dur})
		return nil
	})
	if err != nil {
		return err
	}
	if o.granule == oggNoGranule {
		return nil
	}
	if skipping {
		s.granule, s.prevBlock = o.granule, 0
		return nil
	}
	s.time(o.queue, o.granule, o.flags&ogg.EOS != 0)
	s.granule = o.granule
	return nil
}

// time turns the durations queued in to into the packets' positions:
// back from the page's granule position, where the last packet ends,
// or, on the last page, which may end the stream early, on from the
// page before's.
func (s *oggStream) time(queue []oggPacket, granule int64, last bool) {
	if len(queue) == 0 {
		return
	}
	if last && s.granule != oggNoGranule {
		at := s.granule
		for i := range queue {
			q := &queue[i]
			q.from = at
			at += q.to
			q.to = max(min(at, granule), q.from)
		}
		return
	}
	at := granule
	for i := len(queue) - 1; i >= 0; i-- {
		q := &queue[i]
		q.from, q.to = at-q.to, at
		at = q.from
	}
}

// packetSamples is how many samples packet p adds to its stream.
func (s *oggStream) packetSamples(p []byte) (int64, error) {
	switch s.codec {
	case oggOpus:
		n, ok := opustoc.Samples(p)
		if !ok {
			return 0, fmt.Errorf("an Opus packet of %d bytes without a valid TOC: %w", len(p), ErrMalformed)
		}
		return int64(n), nil
	case oggFLAC:
		h, err := flacstream.ParseHeader(p)
		if err != nil {
			return 0, fmt.Errorf("%w: %w", ErrMalformed, err)
		}
		return int64(h.BlockSize), nil
	case oggVorbis:
		block, err := s.modes.blockSize(p)
		if err != nil {
			return 0, err
		}
		n := 0
		if s.prevBlock > 0 {
			n = s.prevBlock/4 + block/4
		}
		s.prevBlock = block
		return int64(n), nil
	}
	return 0, nil
}

// buildIndex lists every page's offset, stream and granule position.
func (o *oggFile) buildIndex() error {
	for off := int64(0); off < o.src.size; {
		h, err := o.src.view(off, ogg.HeaderSize)
		if err != nil {
			return fmt.Errorf("the Ogg page at offset %d: %w", off, err)
		}
		if string(h[:4]) != oggMagic {
			return fmt.Errorf("no Ogg page at offset %d: %w", off, ErrMalformed)
		}
		nseg := int(h[26])
		page := oggPage{off: off, serial: binary.LittleEndian.Uint32(h[14:]), granule: intconv.Int64Bits(binary.LittleEndian.Uint64(h[6:]))}
		seg, err := o.src.view(off+ogg.HeaderSize, nseg)
		if err != nil {
			return fmt.Errorf("the Ogg page at offset %d: %w", off, err)
		}
		size := int64(ogg.HeaderSize + nseg)
		for _, l := range seg {
			size += int64(l)
		}
		o.index = append(o.index, page)
		off += size
	}
	return nil
}

// seek starts each selected stream after its last page that ends at
// or before the time it is to resume from, the packets on that page
// and the pages before dropped.
func (o *oggFile) seek(track int, at time.Duration, selected []bool) (time.Duration, error) {
	if o.index == nil {
		if err := o.buildIndex(); err != nil {
			return 0, err
		}
	}
	o.restart()
	start := int64(-1)
	seekTo := func(s *oggStream, t time.Duration) (time.Duration, bool) {
		target := intconv.Clamp[int64](timeToTicks(t, s.rate)) + s.offset
		found, last := -1, -1
		for i, p := range o.index {
			if p.serial != s.serial || p.granule == oggNoGranule || p.granule <= s.offset {
				continue
			}
			if p.granule <= target {
				found = i
			}
			last = i
		}
		if found == last {
			found = o.previousPage(s, last)
		}
		if found < 0 {
			return 0, false
		}
		p := o.index[found]
		s.skip, s.skipThrough = 0, p.off
		if start < 0 || p.off < start {
			start = p.off
		}
		return ticksToTime(p.granule-s.offset, s.rate), true
	}
	main := o.streams[track]
	t, ok := seekTo(main, at)
	if !ok {
		o.restart()
		return 0, nil
	}
	landed := t
	for i, s := range o.streams {
		if i == track || !selected[i] {
			continue
		}
		if _, ok := seekTo(s, landed-s.info.SeekPreRoll); !ok {
			o.restart()
			return 0, nil
		}
	}
	o.pos = start
	return landed, nil
}

// previousPage is the index of the page of stream s before the one at
// index i that ends a packet past the stream's start, -1 when there is
// none.
func (o *oggFile) previousPage(s *oggStream, i int) int {
	for i--; i >= 0; i-- {
		if p := o.index[i]; p.serial == s.serial && p.granule != oggNoGranule && p.granule > s.offset {
			return i
		}
	}
	return -1
}

func (o *oggFile) coverArt() []byte { return o.cover.data }
