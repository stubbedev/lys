package glibclocale

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestPaperAndMeasurementAreTheLocales(t *testing.T) {
	f := fixtures(t)
	for _, c := range []struct {
		name          string
		width, height uint32
		measurement   Measurement
	}{
		{"de_DE.UTF-8", 210, 297, Metric},
		{"en_GB.UTF-8", 210, 297, Metric},
		{"en_US.UTF-8", 216, 279, USCustomary},
		{"C", 210, 297, Metric},
	} {
		p, err := f.dirs.Paper(c.name)
		if err != nil {
			t.Fatalf("%s paper: %v", c.name, err)
		}
		if w, h := p.Millimetres(); w != c.width || h != c.height {
			t.Errorf("%s paper = %d×%d mm, want %d×%d", c.name, w, h, c.width, c.height)
		}
		m, err := f.archive.Measurement(c.name)
		if err != nil || m != c.measurement {
			t.Errorf("%s measurement = %d, %v; want %d", c.name, m, err, c.measurement)
		}
	}
}

func testCategory(c category, values ...[]byte) []byte {
	head := make([]byte, categoryHeader+4*len(values))
	binary.LittleEndian.PutUint32(head, c.magic())
	binary.LittleEndian.PutUint32(head[4:], uint32(len(values)))
	var body []byte
	for i, v := range values {
		binary.LittleEndian.PutUint32(head[categoryHeader+4*i:], uint32(len(head)+len(body)))
		body = append(body, v...)
	}
	return append(head, body...)
}

func word(n uint32) []byte { return binary.LittleEndian.AppendUint32(nil, n) }

var codeset = []byte("UTF-8\x00")

func TestAPaperOrMeasurementGlibcCouldNotHaveWrittenIsAnError(t *testing.T) {
	if p, err := parsePaper(testCategory(lcPaper, word(297), word(210), codeset)); err != nil || p != (Paper{width: 210, height: 297}) {
		t.Fatalf("a well-formed paper = %+v, %v", p, err)
	}
	for _, data := range [][]byte{
		testCategory(lcPaper, word(0), word(210), codeset),
		testCategory(lcPaper, word(297), word(maxPaperMillimetre+1), codeset),
		testCategory(lcPaper, word(297)),
		testCategory(lcMeasurement, []byte{1}),
	} {
		if p, err := parsePaper(data); err == nil {
			t.Errorf("% x parsed as paper %+v", data, p)
		}
	}
	if m, err := parseMeasurement(testCategory(lcMeasurement, []byte{2}, codeset)); err != nil || m != USCustomary {
		t.Fatalf("a well-formed measurement = %d, %v", m, err)
	}
	for _, b := range []byte{0, 3} {
		if _, err := parseMeasurement(testCategory(lcMeasurement, []byte{b}, codeset)); err == nil || !strings.Contains(err.Error(), "want 1 or 2") {
			t.Errorf("measurement system %d: %v", b, err)
		}
	}
}
