package procstat

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func TestACommandNameWithSpacesAndParenthesesEndsAtItsLastParenthesis(t *testing.T) {
	got, err := parse([]byte("4242 (a) b (c)) S 1 4242 4242 0 -1 4194560 100 0 0 0 31 11 0 0 20 0 1 0 987654 12345 67 18446744073709551615\n"))
	want := Stat{Comm: "a) b (c)", State: "S", Parent: 1, CPUTicks: 42, StartTime: 987654}
	if err != nil || got != want {
		t.Fatalf("parse = %+v, %v; want %+v", got, err, want)
	}
}

func TestAMalformedStatIsAnError(t *testing.T) {
	for name, bad := range map[string]string{
		"no command name":           "4242 sh S 1 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 100",
		"an unended name":           "4242 (sh S 1 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 100",
		"truncated":                 "4242 (sh) S 1 2 3",
		"no start time":             "4242 (sh) S 1 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0",
		"a user time not a number":  "4242 (sh) S 1 2 3 4 5 6 7 8 9 10 x 11 0 0 20 0 9 0 100",
		"a start time not a number": "4242 (sh) S 1 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 soon",
		"a negative system time":    "4242 (sh) S 1 2 3 4 5 6 7 8 9 10 31 -11 0 0 20 0 9 0 100",
		"a parent not a number":     "4242 (sh) S x 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 100",
		"a parent past a pid":       "4242 (sh) S 4294967296 2 3 4 5 6 7 8 9 10 31 11 0 0 20 0 9 0 100",
	} {
		if got, err := parse([]byte(bad)); err == nil {
			t.Errorf("%s: parse = %+v, want an error", name, got)
		}
	}
}

func TestOfReadsTheKernelsStatOfAProcess(t *testing.T) {
	got, err := Of(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	self, err := Read("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	if got.StartTime == 0 || got.StartTime != self.StartTime || got.Comm != self.Comm || int(got.Parent) != os.Getppid() {
		t.Errorf("Of(%d) = %+v, /proc/self/stat %+v", os.Getpid(), got, self)
	}
	if _, err := Of(-1); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Of(-1): %v, want fs.ErrNotExist", err)
	}
}

func TestCPUIsTheTicksAtTheKernelsClockRate(t *testing.T) {
	tick, err := Stat{CPUTicks: 1}.CPU()
	if err != nil {
		t.Fatal(err)
	}
	if tick <= 0 || time.Second%tick != 0 {
		t.Fatalf("one tick = %s, want a whole fraction of a second", tick)
	}
	if got, err := (Stat{CPUTicks: 250}).CPU(); err != nil || got != 250*tick {
		t.Errorf("250 ticks = %s, %v; want %s", got, err, 250*tick)
	}
	if got, err := (Stat{}).CPU(); err != nil || got != 0 {
		t.Errorf("no ticks = %s, %v; want 0", got, err)
	}
}
