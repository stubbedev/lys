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
	medium            mediumDate
}

// mediumDate is CLDR's medium date pattern: day and month in order,
// the month by its abbreviated name (or its two digits, where the
// pattern is numeric), each followed by its separator, then the year.
type mediumDate struct {
	order      dateOrder
	months     [12]string
	padDay     bool
	separators [2]string
}

func formatsFor(catalogue LangID) (localeFormats, error) {
	switch catalogue.String() {
	case "en-US":
		return localeFormats{
			decimal: ".", group: ",", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: monthDayYear, dateSeparator: "/", hour12: true, dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      monthDayYear,
				months:     [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
				separators: [2]string{" ", ", "},
			},
		}, nil
	case "de":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: ".", padHour: true, dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12"},
				padDay:     true,
				separators: [2]string{".", "."},
			},
		}, nil
	case "es":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sept", "oct", "nov", "dic"},
				separators: [2]string{" ", " "},
			},
		}, nil
	case "fr":
		return localeFormats{
			decimal: ",", group: narrowNBSP, minimumGrouping: 1, units: octetUnitSystems, unitSpace: nbsp,
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: " ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."},
				separators: [2]string{" ", " "},
			},
		}, nil
	case "it":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"gen", "feb", "mar", "apr", "mag", "giu", "lug", "ago", "set", "ott", "nov", "dic"},
				separators: [2]string{" ", " "},
			},
		}, nil
	case "nl":
		return localeFormats{
			decimal: ",", group: ".", minimumGrouping: 1, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "-", padHour: true, dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"jan", "feb", "mrt", "apr", "mei", "jun", "jul", "aug", "sep", "okt", "nov", "dec"},
				separators: [2]string{" ", " "},
			},
		}, nil
	case "pt":
		return localeFormats{
			decimal: ",", group: nbsp, minimumGrouping: 2, units: byteUnitSystems, unitSpace: " ",
			order: dayMonthYear, dateSeparator: "/", padDate: true, padHour: true, dateTimeSeparator: ", ",
			medium: mediumDate{
				order:      dayMonthYear,
				months:     [12]string{"jan.", "fev.", "mar.", "abr.", "mai.", "jun.", "jul.", "ago.", "set.", "out.", "nov.", "dez."},
				separators: [2]string{" de ", " de "},
			},
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

// arrange puts a date's day and month in the order's sequence.
func (o dateOrder) arrange(day, month string) (first, second string) {
	if o == monthDayYear {
		return month, day
	}
	return day, month
}

func (f localeFormats) date(t time.Time) string {
	first, second := f.order.arrange(pad2(t.Day(), f.padDate), pad2(int(t.Month()), f.padDate))
	return first + f.dateSeparator + second + f.dateSeparator + strconv.Itoa(t.Year())
}

func (m mediumDate) format(t time.Time) string {
	first, second := m.order.arrange(pad2(t.Day(), m.padDay), m.months[t.Month()-time.January])
	return first + m.separators[0] + second + m.separators[1] + strconv.Itoa(t.Year())
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

// Date is t's day in t's location, in the locale's CLDR medium date
// pattern, which names the month where the locale does so a day and a
// month never read the wrong way round: "Nov 7, 2026" in en-US,
// "07.11.2026" in de, "7 nov. 2026" in fr, "7 de nov. de 2026" in pt.
func (l *Loader) Date(t time.Time) string { return l.formats().medium.format(t) }

// DateTime is t's numeric date and its time, in t's location:
// "3/4/2026, 5:06 AM" in en-US, "4.3.2026, 05:06" in de, "04/03/2026
// 05:06" in fr.
func (l *Loader) DateTime(t time.Time) string {
	f := l.formats()
	return f.date(t) + f.dateTimeSeparator + f.time(t)
}
