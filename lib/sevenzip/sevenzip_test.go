package sevenzip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/ulikunitz/xz/lzma"
)

// stamp is when every fixture entry was modified.
var stamp = time.Date(2026, time.March, 14, 9, 26, 0, 0, time.UTC)

// fixtureEntries are what testdata/listing.7z and plain.7z hold, as
// 7-Zip wrote them.
var fixtureEntries = []Entry{
	{Name: "empty", Dir: true, Modified: stamp},
	{Name: "src", Dir: true, Modified: stamp},
	{Name: "empty.txt", Modified: stamp},
	{Name: "readme.txt", Size: 12, Modified: stamp},
	{Name: "src/main.go", Size: 13, Modified: stamp},
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func list(data []byte) ([]Entry, error) {
	return List(bytes.NewReader(data), int64(len(data)))
}

// number encodes v as a 7z number.
func number(v uint64) []byte {
	for n := range 8 {
		if v < 1<<(7*(n+1)) {
			out := make([]byte, 1+n)
			out[0] = byte(0xff<<(8-n)) | byte(v>>(8*n))
			for i := range n {
				out[1+i] = byte(v >> (8 * i))
			}
			return out
		}
	}
	return binary.LittleEndian.AppendUint64([]byte{0xff}, v)
}

// withHeader is the archive of the packed streams and the header hdr,
// written as is.
func withHeader(packed, hdr []byte) []byte {
	start := make([]byte, startHeaderSize)
	copy(start, signature)
	start[7] = 4
	binary.LittleEndian.PutUint64(start[12:], uint64(len(packed)))
	binary.LittleEndian.PutUint64(start[20:], uint64(len(hdr)))
	binary.LittleEndian.PutUint32(start[28:], crc32.ChecksumIEEE(hdr))
	binary.LittleEndian.PutUint32(start[8:], crc32.ChecksumIEEE(start[12:]))
	return slices.Concat(start, packed, hdr)
}

// headerOf splits an archive into its packed streams and its header.
func headerOf(t *testing.T, archive []byte) (packed, hdr []byte) {
	t.Helper()
	off, n := binary.LittleEndian.Uint64(archive[12:]), binary.LittleEndian.Uint64(archive[20:])
	return archive[startHeaderSize : startHeaderSize+off], archive[startHeaderSize+off : startHeaderSize+off+n]
}

// lzma2Encoded is archive with its plain header compressed as LZMA2.
func lzma2Encoded(t *testing.T, archive []byte) []byte {
	t.Helper()
	data, hdr := headerOf(t, archive)
	var z bytes.Buffer
	w, err := lzma.Writer2Config{DictCap: lzma.MinDictCap}.NewWriter2(&z)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(hdr); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	enc := slices.Concat([]byte{idEncodedHeader, idPackInfo}, number(uint64(len(data))), number(1), []byte{idSize}, number(uint64(z.Len())), []byte{idEnd},
		[]byte{idUnpackInfo, idFolder}, number(1), []byte{0, 1, 0x21, 0x21, 1, lzma.EncodeDictCap(lzma.MinDictCap), idCodersUnpackSize}, number(uint64(len(hdr))),
		[]byte{idCRC, 1}, binary.LittleEndian.AppendUint32(nil, crc32.ChecksumIEEE(hdr)), []byte{idEnd, idEnd})
	return withHeader(slices.Concat(data, z.Bytes()), enc)
}

// An archive lists its entries in order, folders, an empty file and
// files with their sizes, each modified when it was: with its header
// LZMA-compressed as 7-Zip writes it, plain, and LZMA2-compressed.
func TestAnArchiveListsItsEntries(t *testing.T) {
	plain := fixture(t, "plain.7z")
	for name, data := range map[string][]byte{"listing.7z": fixture(t, "listing.7z"), "plain.7z": plain, "lzma2": lzma2Encoded(t, plain)} {
		got, err := list(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !slices.Equal(got, fixtureEntries) {
			t.Errorf("%s lists %+v, want %+v", name, got, fixtureEntries)
		}
	}
}

// An archive without a header (7-Zip's of nothing) lists nothing.
func TestAnEmptyArchiveListsNothing(t *testing.T) {
	got, err := list(withHeader(nil, nil))
	if err != nil || len(got) != 0 {
		t.Errorf("an empty archive lists %v, %v", got, err)
	}
}

// A number takes as many bytes as its first byte's leading ones say.
func TestNumbersReadTheirLength(t *testing.T) {
	for _, v := range []uint64{0, 1, 0x7f, 0x80, 0x3fff, 0x4000, 1 << 40, 1<<64 - 1} {
		p := &parser{b: number(v)}
		if got := p.number(); got != v || p.err != nil || len(p.b) != 0 {
			t.Errorf("number %#x reads as %#x, %v, %d left", v, got, p.err, len(p.b))
		}
	}
}

// A file that is no archive, a damaged one, one cut short and one whose
// counts run past its header are malformed; one whose header is
// encrypted says so; one whose header a method lys does not decode
// compresses is unsupported.
func TestABrokenArchiveSaysWhy(t *testing.T) {
	plain := fixture(t, "plain.7z")
	data, hdr := headerOf(t, plain)
	flipped := slices.Clone(plain)
	flipped[len(flipped)-3] ^= 0xff
	huge := withHeader(data, slices.Concat([]byte{idHeader, idMainStreamsInfo, idPackInfo, 0}, number(1<<40)))
	unknown := withHeader(data, slices.Concat([]byte{idEncodedHeader, idPackInfo, 0, 1, idSize, 1, idEnd, idUnpackInfo, idFolder, 1, 0, 1, 0x01, 0x04, idCodersUnpackSize, 1, idEnd, idEnd}))
	for name, c := range map[string]struct {
		data []byte
		want error
	}{
		"not an archive":   {[]byte("PK\x03\x04 a zip, really, and long enough for a start header"), ErrMalformed},
		"flipped":          {flipped, ErrMalformed},
		"cut short":        {plain[:len(plain)-10], ErrMalformed},
		"a count past it":  {huge, ErrMalformed},
		"truncated header": {withHeader(data, hdr[:len(hdr)/2]), ErrMalformed},
		"encrypted":        {fixture(t, "encrypted.7z"), ErrEncrypted},
		"unknown method":   {unknown, ErrUnsupported},
	} {
		got, err := list(c.data)
		if !errors.Is(err, c.want) || got != nil {
			t.Errorf("%s lists %v, %v; want %v", name, got, err, c.want)
		}
	}
}
