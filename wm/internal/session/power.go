package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/stubbedev/lys/lib/logind"
)

type LidAction struct {
	sleep    logind.PowerAction
	toLogind bool
}

var DefaultLidAction = LidAction{sleep: logind.AutoSleep}

func LidActions() []LidAction {
	sleeps := logind.SleepActions()
	out := make([]LidAction, 2, len(sleeps)+2)
	out[1] = LidAction{toLogind: true}
	for _, s := range sleeps {
		out = append(out, LidAction{sleep: s})
	}
	return out
}

func (a LidAction) String() string {
	switch {
	case a.toLogind:
		return "logind"
	case a.sleep == 0:
		return "ignore"
	default:
		return a.sleep.String()
	}
}

func (a LidAction) Sleep() (logind.PowerAction, bool) {
	return a.sleep, a.sleep != 0
}

func (a LidAction) LeftToLogind() bool {
	return a.toLogind
}

func OnBattery(roots Roots) (bool, error) {
	battery, err := onBattery(os.DirFS(roots.Sys))
	if err != nil {
		return false, fmt.Errorf("session: read the power supplies under %s: %w", roots.Sys, err)
	}
	return battery, nil
}

func onBattery(sys fs.FS) (bool, error) {
	dir := path.Join("class", string(PowerSupply))
	entries, err := fs.ReadDir(sys, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	external := false
	for _, e := range entries {
		s, err := readSupply(sys, path.Join(dir, e.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if s.online {
			return false, nil
		}
		external = external || s.external
	}
	return external, nil
}

type supply struct {
	external bool
	online   bool
}

func readSupply(sys fs.FS, dir string) (supply, error) {
	kind, err := attribute(sys, dir, "type")
	if err != nil || (kind != "Mains" && kind != "USB") {
		return supply{}, err
	}
	online, err := attribute(sys, dir, "online")
	if err != nil {
		return supply{}, err
	}
	return supply{external: true, online: online == "1"}, nil
}

func attribute(sys fs.FS, dir, name string) (string, error) {
	b, err := fs.ReadFile(sys, path.Join(dir, name))
	return strings.TrimSpace(string(b)), err
}
