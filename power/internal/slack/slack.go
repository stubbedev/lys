// Package slack sets the timer slack of every thread of a user's app, as
// lys-power's half of App Nap: /proc/<tid>/timerslack_ns of another task
// needs CAP_SYS_NICE, which the session lacks.
package slack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/power/internal/meter"
	"github.com/stubbedev/lys/power/internal/sysfs"
	"github.com/stubbedev/lys/power/nap"
)

var ErrNoUnit = errors.New("no such unit in the user's manager")

type Tree struct {
	Cgroup string
	Proc   string
}

func (t Tree) SetTimerSlack(uid uint32, unit nap.Unit, slack time.Duration) error {
	dir, err := t.unitDir(uid, unit)
	if err != nil {
		return err
	}
	tids, err := meter.IDs(dir, "cgroup.threads")
	if err != nil {
		return err
	}
	value := strconv.FormatInt(int64(slack), 10)
	for _, tid := range tids {
		if err := t.set(uid, tid, value); err != nil {
			return err
		}
	}
	return nil
}

func (t Tree) unitDir(uid uint32, unit nap.Unit) (string, error) {
	id := strconv.FormatUint(uint64(uid), 10)
	manager := filepath.Join(t.Cgroup, "user.slice", "user-"+id+".slice", "user@"+id+".service")
	var found string
	err := filepath.WalkDir(manager, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s: %w", path, err)
		}
		if d.IsDir() && d.Name() == string(unit) {
			found = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s under %s: %w", unit, manager, ErrNoUnit)
	}
	return found, nil
}

func (t Tree) set(uid, tid uint32, value string) error {
	task := filepath.Join(t.Proc, strconv.FormatUint(uint64(tid), 10))
	owner, err := realUID(filepath.Join(task, "status"))
	if meter.Gone(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if owner != uid {
		return nil
	}
	err = sysfs.Write(filepath.Join(task, "timerslack_ns"), value)
	if meter.Gone(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("timer slack of thread %d: %w", tid, err)
	}
	return nil
}

func realUID(path string) (uint32, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	for line := range strings.Lines(string(raw)) {
		rest, ok := strings.CutPrefix(line, "Uid:")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			break
		}
		uid, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			return 0, fmt.Errorf("parsing the Uid line of %s: %w", path, err)
		}
		return uint32(uid), nil
	}
	return 0, fmt.Errorf("reading %s: no Uid line", path)
}
