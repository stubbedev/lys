package i18n

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	nbsp       = "\u00a0"
	narrowNBSP = "\u202f"
)

type dateOrder int

const (
	monthDayYear dateOrder = iota
	dayMonthYear
)

type byteUnits [7]string

type unitSystems struct{ binary, decimal byteUnits }

var (
	byteUnitSystems = unitSystems{
		binary:  byteUnits{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"},
		decimal: byteUnits{"B", "kB", "MB", "GB", "TB", "PB", "EB"},
	}
	octetUnitSystems = unitSystems{
		binary:  byteUnits{"o", "Kio", "Mio", "Gio", "Tio", "Pio", "Eio"},
		decimal: byteUnits{"o", "ko", "Mo", "Go", "To", "Po", "Eo"},
	}
	amPMMarkers = [2]string{"AM", "PM"}
)

type localeFormats struct {
	decimal, group    string
	minimumGrouping   int
	units             unitSystems
	unitSpace         string
	order             dateOrder
	dateSeparator     string
	padDate           bool
	hour12            bool
	padHour           bool
	dateTimeSeparator string
}

func formatsFor(catalogue LangID) (localeFormats, error) {
	switch catalogue.String() {
	case "en-US":
		return localeFormats{
			decimal: ".", group: ",", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: monthDayYear, dateSeparator: "/", hour12: true, dateTimeSeparator: ", ",
		}, nil
	case "de":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: ".", padHour: true, dateTimeSeparator: ", ",
		}, nil
	case "es":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", dateTimeSeparator: ", ",
		}, nil
	case "fr":
		return localeFormats{
			decimal: ",", group: narrowNBSP, minimumGrouping: 1, units: octetUnitSystems, unitSpace: nbsp,
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: " ",
		}, nil
	case "it":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: ", ",
		}, nil
	case "nl":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "-", padHour: true, dateTimeSeparator: ", ",
		}, nil
	case "pt":
		return localeFormats{
			decimal: ",", group: nbsp, minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: ", ",
		}, nil
	}
	return localeFormats{}, fmt.Errorf("i18n: catalogue %s has no number and date formats", catalogue)
}

func (f localeFormats) localize(ascii string) string {
	sign, digits := "", ascii
	if rest, ok := strings.CutPrefix(ascii, "-"); ok {
		sign, digits = "-", rest
	}
	whole, fraction, hasFraction := strings.Cut(digits, ".")
	var b strings.Builder
	b.WriteString(sign)
	grouped := len(whole) >= 3+f.minimumGrouping
	for i := range len(whole) {
		if grouped && i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteString(f.group)
		}
		b.WriteByte(whole[i])
	}
	if hasFraction {
		b.WriteString(f.decimal)
		b.WriteString(fraction)
	}
	return b.String()
}

func pad2(n int, pad bool) string {
	if pad && n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (f localeFormats) date(t time.Time) string {
	day, month := pad2(t.Day(), f.padDate), pad2(int(t.Month()), f.padDate)
	first, second := day, month
	if f.order == monthDayYear {
		first, second = month, day
	}
	return first + f.dateSeparator + second + f.dateSeparator + strconv.Itoa(t.Year())
}

func (f localeFormats) time(t time.Time) string {
	minutes := pad2(t.Minute(), true)
	if !f.hour12 {
		return pad2(t.Hour(), f.padHour) + ":" + minutes
	}
	hour := t.Hour() % 12
	if hour == 0 {
		hour = 12
	}
	return pad2(hour, f.padHour) + ":" + minutes + narrowNBSP + amPMMarkers[t.Hour()/12]
}

func (l *Loader) formats() localeFormats { return l.bundles[0].formats }

// Integer is n in the negotiated locale's digits: "10,000" in en-US,
// "10.000" in de, "10 000" (narrow no-break space) in fr.
func (l *Loader) Integer(n int64) string {
	return l.formats().localize(strconv.FormatInt(n, 10))
}

// byteSize is n in powers of base named by units, with one decimal,
// plain bytes below base. A size that would round up to base steps to
// the next unit, so 1,048,575 bytes are "1.0 MiB", never "1024.0 KiB".
func (f localeFormats) byteSize(n, base uint64, units byteUnits) string {
	if n < base {
		return f.localize(strconv.FormatUint(n, 10)) + f.unitSpace + units[0]
	}
	size, unit := float64(n), 0
	for unit < len(units)-1 && math.Round(size*10)/10 >= float64(base) {
		size /= float64(base)
		unit++
	}
	return f.localize(strconv.FormatFloat(size, 'f', 1, 64)) + f.unitSpace + units[unit]
}

// ByteSize is a byte count in binary (1024-based) units with one
// decimal, plain bytes below a kibibyte: "1.5 KiB" in en-US, "1,5 KiB"
// in de, "1,5 Kio" in fr. Memory and files are measured so.
func (l *Loader) ByteSize(n uint64) string {
	f := l.formats()
	return f.byteSize(n, 1024, f.units.binary)
}

// DecimalByteSize is a byte count in decimal (1000-based) units with
// one decimal, plain bytes below a kilobyte: "500.1 GB" in en-US,
// "500,1 GB" in de, "500,1 Go" in fr. Drives are sold so.
func (l *Loader) DecimalByteSize(n uint64) string {
	f := l.formats()
	return f.byteSize(n, 1000, f.units.decimal)
}

// Time is t's hour and minute in t's location, in the locale's hour
// cycle: "5:06 AM" in en-US, "05:06" in de and fr.
func (l *Loader) Time(t time.Time) string { return l.formats().time(t) }

// DateTime is t's numeric date and its time, in t's location:
// "3/4/2026, 5:06 AM" in en-US, "4.3.2026, 05:06" in de, "04/03/2026
// 05:06" in fr.
func (l *Loader) DateTime(t time.Time) string {
	f := l.formats()
	return f.date(t) + f.dateTimeSeparator + f.time(t)
}
