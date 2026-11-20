package geo

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNewCoordinatesRefusesWhatIsNotAPosition(t *testing.T) {
	for _, tc := range []struct {
		lat, lng float64
		want     string
	}{
		{90.5, 10, "latitude 90.5"},
		{-90.5, 10, "latitude -90.5"},
		{math.NaN(), 10, "latitude NaN"},
		{10, 180.5, "longitude 180.5"},
		{10, math.NaN(), "longitude NaN"},
		{0, 0, "0, 0"},
	} {
		if _, err := NewCoordinates(tc.lat, tc.lng); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("NewCoordinates(%v, %v) = %v, want an error naming %q", tc.lat, tc.lng, err, tc.want)
		}
	}
	if _, err := NewCoordinates(0, 0); !errors.Is(err, ErrNullIsland) {
		t.Errorf("0, 0 = %v, want ErrNullIsland", err)
	}
	for _, ok := range [][2]float64{{90, 180}, {-90, -180}, {0, 12.5}, {55.6, 0}} {
		c, err := NewCoordinates(ok[0], ok[1])
		if err != nil || c.Latitude() != ok[0] || c.Longitude() != ok[1] {
			t.Errorf("NewCoordinates(%v, %v) = %+v, %v", ok[0], ok[1], c, err)
		}
	}
}

func TestCoordinatesReadAsLatitudeCommaLongitude(t *testing.T) {
	c, err := NewCoordinates(55.6761, -12.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.String(); got != "55.6761,-12.5" {
		t.Errorf("String() = %q", got)
	}
}

func TestAPlaceKeepsItsTrimmedName(t *testing.T) {
	c, _ := NewCoordinates(1, 2)
	p := NewPlace("  Oslo ", c)
	if p.Name() != "Oslo" || p.Coordinates() != c {
		t.Errorf("NewPlace = %q at %v", p.Name(), p.Coordinates())
	}
	if NewPlace("", c).Name() != "" {
		t.Error("an unnamed place gained a name")
	}
}

func TestParseISO6709ReadsTheZoneTableForms(t *testing.T) {
	for s, want := range map[string][2]float64{
		"+5540+01235":     {55 + 40.0/60, 12 + 35.0/60},
		"+404251-0740023": {40 + 42.0/60 + 51.0/3600, -(74 + 0.0/60 + 23.0/3600)},
		"-3352+15113":     {-(33 + 52.0/60), 151 + 13.0/60},
	} {
		c, err := ParseISO6709(s)
		if err != nil {
			t.Errorf("%s: %v", s, err)
			continue
		}
		if math.Abs(c.Latitude()-want[0]) > 1e-9 || math.Abs(c.Longitude()-want[1]) > 1e-9 {
			t.Errorf("%s = %v, %v, want %v", s, c.Latitude(), c.Longitude(), want)
		}
	}
}

func TestParseISO6709RefusesMalformedPositions(t *testing.T) {
	for _, s := range []string{"", "5540+01235", "+5540", "+554+01235", "+5560+01235", "+5540+0123", "+55a0+01235", "+0000+00000", "+9500+01235"} {
		if _, err := ParseISO6709(s); err == nil {
			t.Errorf("ParseISO6709(%q) was accepted", s)
		}
	}
}
