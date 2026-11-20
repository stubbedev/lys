// Package sleep is lys-power's report on each sleep the system took: how
// much of it was spent in the platform's deepest idle state, the battery it
// drained, the devices that kept it shallow and those that woke it, and the
// system-bus contract that publishes the last few reports as the Sleeps
// property of dev.stubbe.lys.Sleep1, with its PropertiesChanged.
package sleep

import (
	"fmt"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName = "dev.stubbe.lys.Sleep1"
	ServicePath = dbus.ObjectPath("/dev/stubbe/lys/Sleep")
	Interface   = "dev.stubbe.lys.Sleep1"
	Property    = "Sleeps"
)

const (
	// MinSleep is the shortest sleep judged: below it the battery's energy
	// counter and the residency counter are too coarse to say anything.
	MinSleep = 10 * time.Minute
	// DeepShare is the least share of a sleep spent in the deepest idle
	// state that counts as deep.
	DeepShare = 0.9
	// Kept is how many reports lys-power keeps.
	Kept = 10
)

// Depth is how deeply a sleep slept.
type Depth uint8

// The depths. DepthUnmeasured is a sleep too short to judge, on a platform
// without a residency counter, or one whose counter started over, as
// after a hibernation.
const (
	DepthUnmeasured Depth = iota + 1
	DepthDeep
	DepthShallow
)

var depthNames = [...]string{DepthUnmeasured: "unmeasured", DepthDeep: "deep", DepthShallow: "shallow"}

func (d Depth) String() string {
	if d < DepthUnmeasured || int(d) >= len(depthNames) {
		return fmt.Sprintf("Depth(%d)", uint8(d))
	}
	return depthNames[d]
}

// ParseDepth reads a depth as String names it.
func ParseDepth(s string) (Depth, error) {
	i := slices.Index(depthNames[:], s)
	if i < int(DepthUnmeasured) {
		return 0, fmt.Errorf("sleep depth %q: want unmeasured, deep or shallow", s)
	}
	return intconv.Must[Depth](i), nil
}

// Drain is the battery power a sleep drew, when it is known: the sleep
// started and ended on battery, long enough to measure.
type Drain struct {
	milliwatts uint64
	known      bool
}

// Drew is a known drain.
func Drew(milliwatts uint64) Drain { return Drain{milliwatts: milliwatts, known: true} }

// Milliwatts is the drain, and whether it is known.
func (d Drain) Milliwatts() (uint64, bool) { return d.milliwatts, d.known }

// Report is one sleep. DeepShare is the share of it spent in the deepest
// idle state, meaningful unless Depth is DepthUnmeasured. Blockers are
// the platform's requirements for that state left unmet, WokenBy the
// wakeup sources that fired during it, most first.
type Report struct {
	Slept     time.Time
	Woke      time.Time
	Depth     Depth
	DeepShare float64
	Drain     Drain
	Blockers  []string
	WokenBy   []string
}

// Asleep is how long the sleep lasted.
func (r Report) Asleep() time.Duration { return r.Woke.Sub(r.Slept) }

// Drained is the drain of a sleep worth telling the user about: one that
// stayed shallow while it drew on the battery.
func (r Report) Drained() (milliwatts uint64, ok bool) {
	mw, known := r.Drain.Milliwatts()
	return mw, known && r.Depth == DepthShallow
}

// Wire is a report as it travels on the bus and is kept on disk.
type Wire struct {
	Slept           int64
	Woke            int64
	Depth           string
	DeepShare       float64
	DrainMilliwatts uint64
	DrainKnown      bool
	Blockers        []string
	WokenBy         []string
}

// ToWire is r on the wire.
func ToWire(r Report) Wire {
	mw, known := r.Drain.Milliwatts()
	return Wire{
		Slept: r.Slept.UnixMicro(), Woke: r.Woke.UnixMicro(), Depth: r.Depth.String(), DeepShare: r.DeepShare,
		DrainMilliwatts: mw, DrainKnown: known, Blockers: nonNil(r.Blockers), WokenBy: nonNil(r.WokenBy),
	}
}

// FromWire reads a report off the wire.
func FromWire(w Wire) (Report, error) {
	depth, err := ParseDepth(w.Depth)
	if err != nil {
		return Report{}, err
	}
	slept, woke := time.UnixMicro(w.Slept).UTC(), time.UnixMicro(w.Woke).UTC()
	if woke.Before(slept) {
		return Report{}, fmt.Errorf("sleep woke at %s before it slept at %s", woke, slept)
	}
	if w.DeepShare < 0 || w.DeepShare > 1 {
		return Report{}, fmt.Errorf("sleep from %s: deep share %v is not between 0 and 1", slept, w.DeepShare)
	}
	if !w.DrainKnown && w.DrainMilliwatts != 0 {
		return Report{}, fmt.Errorf("sleep from %s: a drain of %d mW marked unknown", slept, w.DrainMilliwatts)
	}
	r := Report{Slept: slept, Woke: woke, Depth: depth, DeepShare: w.DeepShare, Blockers: orNil(w.Blockers), WokenBy: orNil(w.WokenBy)}
	if w.DrainKnown {
		r.Drain = Drew(w.DrainMilliwatts)
	}
	return r, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func orNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func toWires(rs []Report) []Wire {
	out := make([]Wire, len(rs))
	for i, r := range rs {
		out[i] = ToWire(r)
	}
	return out
}

func fromWires(ws []Wire) ([]Report, error) {
	out := make([]Report, len(ws))
	for i, w := range ws {
		r, err := FromWire(w)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}
