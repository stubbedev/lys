// Package nap is App Nap for lys: which app is napping, given what its
// windows, streams and inhibitors say, and what that costs it in cgroup
// weights, timer slack, cores and freezing. It decides only; the cgroup
// and timer slack writes are the caller's effect, and the system-bus
// contract that lets an unprivileged session set the slack of its own
// apps' threads is in this package too.
package nap

import (
	"fmt"
	"math"
	"time"
)

type Level uint8

const (
	LevelDefault Level = iota + 1
	LevelNever
	LevelEfficiencyCores
	LevelFrozen
)

var levelNames = [...]string{LevelDefault: "default", LevelNever: "never", LevelEfficiencyCores: "efficiency-cores", LevelFrozen: "frozen"}

func Levels() []Level {
	return []Level{LevelDefault, LevelNever, LevelEfficiencyCores, LevelFrozen}
}

func (l Level) String() string {
	if l < LevelDefault || l > LevelFrozen {
		return fmt.Sprintf("Level(%d)", uint8(l))
	}
	return levelNames[l]
}

func (l Level) MarshalText() ([]byte, error) {
	if _, err := ParseLevel(l.String()); err != nil {
		return nil, err
	}
	return []byte(l.String()), nil
}

func (l *Level) UnmarshalText(text []byte) error {
	parsed, err := ParseLevel(string(text))
	if err != nil {
		return err
	}
	*l = parsed
	return nil
}

func ParseLevel(name string) (Level, error) {
	for _, l := range Levels() {
		if levelNames[l] == name {
			return l, nil
		}
	}
	return 0, fmt.Errorf("nap level %q: want default, never, efficiency-cores or frozen", name)
}

type Supply uint8

const (
	SupplyMains Supply = iota + 1
	SupplyBattery
)

type Activity struct {
	Windows  int
	Seen     int
	Focused  bool
	Streams  bool
	Inhibits bool
}

func (a Activity) Napping() bool {
	return a.Windows > 0 && a.Seen == 0 && !a.Focused && !a.Streams && !a.Inhibits
}

type Weight uint64

const (
	WeightUnset   Weight = math.MaxUint64
	NappingWeight Weight = 20
)

type Cores uint8

const (
	CoresAll Cores = iota + 1
	CoresEfficiency
)

type Treatment struct {
	CPUWeight  Weight
	IOWeight   Weight
	TimerSlack time.Duration
	Cores      Cores
	Frozen     bool
}

const NappingSlack = 50 * time.Millisecond

var (
	Awake  = Treatment{CPUWeight: WeightUnset, IOWeight: WeightUnset, Cores: CoresAll}
	Napped = Treatment{CPUWeight: NappingWeight, IOWeight: NappingWeight, TimerSlack: NappingSlack, Cores: CoresAll}
)

func Decide(a Activity, l Level, s Supply) Treatment {
	if l == LevelNever || !a.Napping() {
		return Awake
	}
	t := Napped
	switch l {
	case LevelEfficiencyCores:
		t.Cores = CoresEfficiency
	case LevelFrozen:
		t.Frozen = s == SupplyBattery
	}
	return t
}
