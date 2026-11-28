// Package sevenzip lists what a 7z archive holds from its header alone:
// each entry's name, whether it is a folder, its size and when it was
// modified. A header compressed with LZMA or LZMA2, as 7-Zip writes it,
// is decompressed; no file's contents are read.
package sevenzip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"time"
	"unicode/utf16"

	"github.com/ulikunitz/xz/lzma"
)

// HeaderBytes is the largest header List reads, compressed or not.
const HeaderBytes = 64 << 20

// The errors List returns unwrap to one of these: a file that is no 7z
// archive or whose header does not parse, a header encrypted with a
// password, and a header compressed with a method lys does not decode.
var (
	ErrMalformed   = errors.New("sevenzip: malformed")
	ErrEncrypted   = errors.New("sevenzip: the header is encrypted")
	ErrUnsupported = errors.New("sevenzip: unsupported")
)

// Entry is one file or folder an archive holds.
type Entry struct {
	Name     string
	Dir      bool
	Size     int64
	Modified time.Time
}

const (
	signature       = "7z\xbc\xaf\x27\x1c"
	startHeaderSize = 32
	maxEncodings    = 4
	attrDirectory   = 0x10
	filetimeEpoch   = 11644473600
)

const (
	idEnd = iota
	idHeader
	idArchiveProperties
	idAdditionalStreamsInfo
	idMainStreamsInfo
	idFilesInfo
	idPackInfo
	idUnpackInfo
	idSubStreamsInfo
	idSize
	idCRC
	idFolder
	idCodersUnpackSize
	idNumUnpackStream
	idEmptyStream
	idEmptyFile
	idAnti
	idName
	idCTime
	idATime
	idMTime
	idWinAttributes
	idComment
	idEncodedHeader
)

// The header coders decoded, and AES, which marks an encrypted header.
const (
	methodCopy  = "\x00"
	methodLZMA  = "\x03\x01\x01"
	methodLZMA2 = "\x21"
	methodAES   = "\x06\xf1\x07\x01"
)

// List is every entry of the 7z archive r holds, size bytes long, in
// the order its header lists them.
func List(r io.ReaderAt, size int64) ([]Entry, error) {
	var start [startHeaderSize]byte
	if _, err := r.ReadAt(start[:], 0); err != nil {
		return nil, fmt.Errorf("%w: reading the signature header: %w", ErrMalformed, err)
	}
	if string(start[:len(signature)]) != signature {
		return nil, fmt.Errorf("%w: no 7z signature", ErrMalformed)
	}
	if crc32.ChecksumIEEE(start[12:]) != binary.LittleEndian.Uint32(start[8:]) {
		return nil, fmt.Errorf("%w: the start header's checksum does not match", ErrMalformed)
	}
	off, n := binary.LittleEndian.Uint64(start[12:]), binary.LittleEndian.Uint64(start[20:])
	if n == 0 {
		return nil, nil
	}
	buf, err := readSection(r, size, off, n)
	if err != nil {
		return nil, err
	}
	if crc32.ChecksumIEEE(buf) != binary.LittleEndian.Uint32(start[28:]) {
		return nil, fmt.Errorf("%w: the header's checksum does not match", ErrMalformed)
	}
	for range maxEncodings {
		p := &parser{b: buf}
		switch id := p.number(); id {
		case idHeader:
			entries := p.header()
			if p.err != nil {
				return nil, p.err
			}
			return entries, nil
		case idEncodedHeader:
			if buf, err = decodeHeader(r, size, p); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%w: a header starting with property %d", ErrMalformed, id)
		}
	}
	return nil, fmt.Errorf("%w: a header encoded more than %d times", ErrMalformed, maxEncodings)
}

// readSection reads the n bytes at off past the start header of the
// archive r holds, size bytes long, at most HeaderBytes.
func readSection(r io.ReaderAt, size int64, off, n uint64) ([]byte, error) {
	if n > HeaderBytes {
		return nil, fmt.Errorf("%w: a header of %d bytes, past the %d read", ErrUnsupported, n, HeaderBytes)
	}
	if size < startHeaderSize || off > uint64(size-startHeaderSize) || n > uint64(size-startHeaderSize)-off {
		return nil, fmt.Errorf("%w: %d bytes at %d past the archive's %d", ErrMalformed, n, off, size)
	}
	buf := make([]byte, n)
	if _, err := r.ReadAt(buf, startHeaderSize+int64(off)); err != nil { //nolint:gosec // off is bounded by size above
		return nil, fmt.Errorf("%w: reading %d header bytes at %d: %w", ErrMalformed, n, off, err)
	}
	return buf, nil
}

// decodeHeader decompresses the header the streams p is at describe.
func decodeHeader(r io.ReaderAt, size int64, p *parser) ([]byte, error) {
	s := p.streamsInfo()
	if p.err != nil {
		return nil, p.err
	}
	if len(s.folders) != 1 || len(s.packSizes) != 1 || len(s.folders[0].coders) != 1 {
		return nil, fmt.Errorf("%w: an encoded header of %d folders and %d packed streams", ErrUnsupported, len(s.folders), len(s.packSizes))
	}
	f := s.folders[0]
	packed, err := readSection(r, size, s.packPos, s.packSizes[0])
	if err != nil {
		return nil, err
	}
	n := f.unpackSize()
	if n > HeaderBytes {
		return nil, fmt.Errorf("%w: a header of %d bytes, past the %d read", ErrUnsupported, n, HeaderBytes)
	}
	src, err := decoder(f.coders[0], packed, n)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(src, buf); err != nil {
		return nil, fmt.Errorf("%w: decompressing the header: %w", ErrMalformed, err)
	}
	if f.hasCRC && crc32.ChecksumIEEE(buf) != f.crc {
		return nil, fmt.Errorf("%w: the decompressed header's checksum does not match", ErrMalformed)
	}
	return buf, nil
}

// decoder reads the n bytes c decodes from packed.
func decoder(c coder, packed []byte, n uint64) (io.Reader, error) {
	src := bytes.NewReader(packed)
	switch string(c.method) {
	case methodCopy:
		return src, nil
	case methodLZMA:
		if len(c.props) != 5 {
			return nil, fmt.Errorf("%w: LZMA properties of %d bytes", ErrMalformed, len(c.props))
		}
		head := binary.LittleEndian.AppendUint64(c.props[:5:5], n)
		lr, err := lzma.NewReader(io.MultiReader(bytes.NewReader(head), src))
		if err != nil {
			return nil, fmt.Errorf("%w: the header's LZMA stream: %w", ErrMalformed, err)
		}
		return lr, nil
	case methodLZMA2:
		if len(c.props) != 1 {
			return nil, fmt.Errorf("%w: LZMA2 properties of %d bytes", ErrMalformed, len(c.props))
		}
		dict, err := lzma.DecodeDictCap(c.props[0])
		if err != nil {
			return nil, fmt.Errorf("%w: the header's LZMA2 dictionary: %w", ErrMalformed, err)
		}
		lr, err := lzma.Reader2Config{DictCap: int(max(min(dict, int64(n)), lzma.MinDictCap))}.NewReader2(src) //nolint:gosec // n is at most HeaderBytes
		if err != nil {
			return nil, fmt.Errorf("%w: the header's LZMA2 stream: %w", ErrMalformed, err)
		}
		return lr, nil
	case methodAES:
		return nil, ErrEncrypted
	}
	return nil, fmt.Errorf("%w: a header compressed with method %x", ErrUnsupported, c.method)
}

// streams is what a streams info block says: where the packed streams
// start and how long each is, the folders unpacking them, and the size
// of every stream they unpack to, in order.
type streams struct {
	packPos   uint64
	packSizes []uint64
	folders   []folder
	sizes     []uint64
}

// folder is a chain of coders, the sizes of what each of their outputs
// unpacks to, which of them is the folder's own (main), its checksum
// where it has one, and how many streams it holds.
type folder struct {
	coders   []coder
	outSizes []uint64
	main     int
	crc      uint32
	hasCRC   bool
	streams  int
}

func (f folder) unpackSize() uint64 { return f.outSizes[f.main] }

// coder is one method a folder decodes with, its properties and how
// many streams it takes and gives.
type coder struct {
	method  []byte
	props   []byte
	in, out int
}

// parser reads a header, holding the first error it meets; every read
// after one gives zero values.
type parser struct {
	b   []byte
	err error
}

func (p *parser) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf("%w: "+format, append([]any{ErrMalformed}, args...)...)
	}
}

func (p *parser) byte() byte {
	if p.err != nil {
		return 0
	}
	if len(p.b) == 0 {
		p.fail("the header ends early")
		return 0
	}
	c := p.b[0]
	p.b = p.b[1:]
	return c
}

func (p *parser) bytes(n uint64) []byte {
	if p.err != nil {
		return nil
	}
	if n > uint64(len(p.b)) {
		p.fail("%d bytes past the header's end", n)
		return nil
	}
	out := p.b[:n]
	p.b = p.b[n:]
	return out
}

// number reads a 7z number: the leading one bits of its first byte
// count the little-endian bytes that follow, its remaining bits the
// value's high bits.
func (p *parser) number() uint64 {
	first := p.byte()
	var v uint64
	mask := byte(0x80)
	for i := range 8 {
		if first&mask == 0 {
			return v | uint64(first&(mask-1))<<(8*i)
		}
		v |= uint64(p.byte()) << (8 * i)
		mask >>= 1
	}
	return v
}

// count reads a number of items that each take at least a byte of
// what remains of the header, so no corrupt count allocates past it.
func (p *parser) count() int {
	n := p.number()
	if n > uint64(len(p.b)) {
		p.fail("a count of %d in %d header bytes", n, len(p.b))
		return 0
	}
	return int(n) //nolint:gosec // bounded by len(p.b)
}

func (p *parser) uint32() uint32 {
	b := p.bytes(4)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

func (p *parser) uint64() uint64 {
	b := p.bytes(8)
	if b == nil {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func (p *parser) expect(id uint64) {
	if got := p.number(); got != id && p.err == nil {
		p.fail("property %d where %d belongs", got, id)
	}
}

// bits reads n flags, the first in a byte's high bit.
func (p *parser) bits(n int) []bool {
	b := p.bytes(uint64((n + 7) / 8)) //nolint:gosec // n is non-negative
	if b == nil {
		return make([]bool, n)
	}
	out := make([]bool, n)
	for i := range out {
		out[i] = b[i/8]&(0x80>>(i%8)) != 0
	}
	return out
}

// defined reads which of n items are defined: all of them, or each as
// its flag says.
func (p *parser) defined(n int) []bool {
	if p.byte() == 0 {
		return p.bits(n)
	}
	out := make([]bool, n)
	for i := range out {
		out[i] = true
	}
	return out
}

// digests reads the checksums of n items, which of them are defined
// first.
func (p *parser) digests(n int) ([]bool, []uint32) {
	def := p.defined(n)
	sums := make([]uint32, n)
	for i, d := range def {
		if d {
			sums[i] = p.uint32()
		}
	}
	return def, sums
}

// sub is a parser of the next n bytes, which p moves past.
func (p *parser) sub(n uint64) *parser {
	b := p.bytes(n)
	return &parser{b: b, err: p.err}
}

// header reads a header past its id: the streams of its files and the
// files themselves.
func (p *parser) header() []Entry {
	id := p.number()
	if id == idArchiveProperties {
		for p.err == nil && p.byte() != 0 {
			p.bytes(p.number())
		}
		id = p.number()
	}
	if id == idAdditionalStreamsInfo {
		p.streamsInfo()
		id = p.number()
	}
	var main streams
	if id == idMainStreamsInfo {
		main = p.streamsInfo()
		id = p.number()
	}
	var entries []Entry
	if id == idFilesInfo {
		entries = p.filesInfo(main.sizes)
		id = p.number()
	}
	if id != idEnd {
		p.fail("property %d where the header ends", id)
	}
	return entries
}

func (p *parser) streamsInfo() streams {
	var s streams
	unpacked := false
	for id := p.number(); id != idEnd && p.err == nil; id = p.number() {
		switch id {
		case idPackInfo:
			s.packPos = p.number()
			s.packSizes = make([]uint64, p.count())
			for id := p.number(); id != idEnd && p.err == nil; id = p.number() {
				switch id {
				case idSize:
					for i := range s.packSizes {
						s.packSizes[i] = p.number()
					}
				case idCRC:
					p.digests(len(s.packSizes))
				default:
					p.fail("property %d in the pack info", id)
				}
			}
		case idUnpackInfo:
			s.folders = p.unpackInfo()
		case idSubStreamsInfo:
			s.sizes = p.subStreamsInfo(s.folders)
			unpacked = true
		default:
			p.fail("property %d in a streams info", id)
		}
	}
	if !unpacked && p.err == nil {
		for i := range s.folders {
			s.folders[i].streams = 1
			s.sizes = append(s.sizes, s.folders[i].unpackSize())
		}
	}
	return s
}

func (p *parser) unpackInfo() []folder {
	p.expect(idFolder)
	folders := make([]folder, p.count())
	if p.byte() != 0 {
		p.fail("folders kept outside the header")
	}
	for i := range folders {
		folders[i] = p.folder()
	}
	p.expect(idCodersUnpackSize)
	for i := range folders {
		for j := range folders[i].outSizes {
			folders[i].outSizes[j] = p.number()
		}
	}
	for id := p.number(); id != idEnd && p.err == nil; id = p.number() {
		if id != idCRC {
			p.fail("property %d in the unpack info", id)
			break
		}
		def, sums := p.digests(len(folders))
		for i := range folders {
			folders[i].hasCRC, folders[i].crc = def[i], sums[i]
		}
	}
	return folders
}

func (p *parser) folder() folder {
	var f folder
	f.coders = make([]coder, p.count())
	var in, out int
	for i := range f.coders {
		flags := p.byte()
		if flags&0x80 != 0 {
			p.fail("a coder with alternative methods")
			return f
		}
		c := coder{method: p.bytes(uint64(flags & 0x0f)), in: 1, out: 1}
		if flags&0x10 != 0 {
			c.in, c.out = p.count(), p.count()
		}
		if flags&0x20 != 0 {
			c.props = p.bytes(p.number())
		}
		in, out = in+c.in, out+c.out
		f.coders[i] = c
	}
	if p.err != nil {
		return f
	}
	if out == 0 || in < out-1 {
		p.fail("a folder of %d inputs and %d outputs", in, out)
		return f
	}
	bound := make([]bool, out)
	for range out - 1 {
		p.number()
		o := p.number()
		if o >= uint64(out) {
			p.fail("a bind pair to output %d of %d", o, out)
			return f
		}
		bound[o] = true
	}
	if packed := in - (out - 1); packed > 1 {
		for range packed {
			p.number()
		}
	}
	f.outSizes = make([]uint64, out)
	f.main = -1
	for i, b := range bound {
		if !b {
			f.main = i
			break
		}
	}
	if f.main < 0 {
		p.fail("a folder whose every output is bound")
	}
	return f
}

// subStreamsInfo reads how many streams each folder holds and their
// sizes, the last of a folder's what its others leave of its size.
func (p *parser) subStreamsInfo(folders []folder) []uint64 {
	for i := range folders {
		folders[i].streams = 1
	}
	id := p.number()
	if id == idNumUnpackStream {
		total := 0
		for i := range folders {
			folders[i].streams = p.count()
			total += folders[i].streams
			if total > len(p.b)+len(folders) {
				p.fail("%d streams in %d header bytes", total, len(p.b))
				return nil
			}
		}
		id = p.number()
	}
	sized := id == idSize
	var sizes []uint64
	for _, f := range folders {
		if f.streams == 0 || p.err != nil {
			continue
		}
		var sum uint64
		for range f.streams - 1 {
			if !sized {
				p.fail("a folder of %d streams without their sizes", f.streams)
				return nil
			}
			n := p.number()
			sizes = append(sizes, n)
			sum += n
		}
		if sum > f.unpackSize() {
			p.fail("streams of %d bytes in a folder of %d", sum, f.unpackSize())
			return nil
		}
		sizes = append(sizes, f.unpackSize()-sum)
	}
	if sized {
		id = p.number()
	}
	if id == idCRC {
		unknown := 0
		for _, f := range folders {
			if f.streams != 1 || !f.hasCRC {
				unknown += f.streams
			}
		}
		p.digests(unknown)
		id = p.number()
	}
	if id != idEnd {
		p.fail("property %d in the substreams info", id)
	}
	return sizes
}

// filesInfo reads the files, those with contents taking their sizes
// from sizes in order.
func (p *parser) filesInfo(sizes []uint64) []Entry {
	entries := make([]Entry, p.count())
	var empty, emptyFile, dir []bool
	for id := p.number(); id != idEnd && p.err == nil; id = p.number() {
		q := p.sub(p.number())
		switch id {
		case idEmptyStream:
			empty = q.bits(len(entries))
			emptyFile = make([]bool, countTrue(empty))
		case idEmptyFile:
			emptyFile = q.bits(countTrue(empty))
		case idName:
			q.names(entries)
		case idMTime:
			def := q.defined(len(entries))
			q.external()
			for i, d := range def {
				if d {
					entries[i].Modified = filetime(q.uint64())
				}
			}
		case idWinAttributes:
			def := q.defined(len(entries))
			q.external()
			dir = make([]bool, len(entries))
			for i, d := range def {
				if d {
					dir[i] = q.uint32()&attrDirectory != 0
				}
			}
		}
		if q.err != nil {
			p.err = q.err
		}
	}
	if p.err != nil {
		return nil
	}
	stream, emptyIndex := 0, 0
	for i := range entries {
		e := &entries[i]
		if dir != nil {
			e.Dir = dir[i]
		}
		if empty != nil && empty[i] {
			e.Dir = e.Dir || !emptyFile[emptyIndex]
			emptyIndex++
			continue
		}
		if stream == len(sizes) {
			p.fail("%d files with contents in %d streams", stream+1, len(sizes))
			return nil
		}
		e.Size = int64(min(sizes[stream], 1<<63-1))
		stream++
	}
	return entries
}

// external reads the flag saying a property's values are kept outside
// the header, which no archiver writes.
func (p *parser) external() {
	if p.byte() != 0 {
		p.fail("file properties kept outside the header")
	}
}

// names reads the entries' names: UTF-16LE, each ending in a zero.
func (p *parser) names(entries []Entry) {
	p.external()
	for i := range entries {
		var units []uint16
		for {
			b := p.bytes(2)
			if b == nil {
				return
			}
			u := binary.LittleEndian.Uint16(b)
			if u == 0 {
				break
			}
			units = append(units, u)
		}
		entries[i].Name = string(utf16.Decode(units))
	}
}

// filetime is a Windows FILETIME, 100 nanoseconds since 1601, in UTC.
func filetime(ft uint64) time.Time {
	return time.Unix(int64(ft/1e7)-filetimeEpoch, int64(ft%1e7)*100).UTC() //nolint:gosec // ft/1e7 fits an int64
}

func countTrue(flags []bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}
