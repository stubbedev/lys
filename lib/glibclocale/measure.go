package glibclocale

import "fmt"

const (
	paperHeight        = item(lcPaper) << 16
	paperWidth         = paperHeight + 1
	measurementSystem  = item(lcMeasurement) << 16
	maxPaperMillimetre = 10000
)

// Paper is a locale's LC_PAPER: the size of the paper it prints on.
type Paper struct{ width, height uint32 }

// Millimetres are the paper's width and height.
func (p Paper) Millimetres() (width, height uint32) { return p.width, p.height }

// Measurement is a locale's LC_MEASUREMENT: the units it measures in.
type Measurement uint8

// The two systems of measurement glibc knows.
const (
	Metric      Measurement = 1
	USCustomary Measurement = 2
)

var (
	cPaper       = Paper{width: 210, height: 297}
	cMeasurement = Metric
)

func parsePaper(data []byte) (Paper, error) {
	d, err := parseCategory(lcPaper, data, paperWidth)
	if err != nil {
		return Paper{}, err
	}
	var p Paper
	if p.height, err = d.word(paperHeight); err != nil {
		return Paper{}, err
	}
	if p.width, err = d.word(paperWidth); err != nil {
		return Paper{}, err
	}
	if p.width == 0 || p.height == 0 || p.width > maxPaperMillimetre || p.height > maxPaperMillimetre {
		return Paper{}, fmt.Errorf("%s has a paper of %d×%d mm", lcPaper, p.width, p.height)
	}
	return p, nil
}

func parseMeasurement(data []byte) (Measurement, error) {
	d, err := parseCategory(lcMeasurement, data, measurementSystem)
	if err != nil {
		return 0, err
	}
	c, err := d.char(measurementSystem)
	if err != nil {
		return 0, err
	}
	switch m := Measurement(c); m {
	case Metric, USCustomary:
		return m, nil
	}
	return 0, fmt.Errorf("%s has measurement system %d, want 1 or 2", lcMeasurement, c)
}
