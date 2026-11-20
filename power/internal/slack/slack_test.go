package slack

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/nap"
)

const user = 1000

type fake struct {
	t    *testing.T
	tree Tree
}

func newFake(t *testing.T) fake {
	t.Helper()
	dir := t.TempDir()
	return fake{t: t, tree: Tree{Cgroup: filepath.Join(dir, "cgroup"), Proc: filepath.Join(dir, "proc")}}
}

func (f fake) write(path, body string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f fake) unit(uid uint32, rel string, tids ...int) {
	f.t.Helper()
	id := strconv.FormatUint(uint64(uid), 10)
	lines := make([]string, len(tids))
	for i, tid := range tids {
		lines[i] = strconv.Itoa(tid)
	}
	f.write(filepath.Join(f.tree.Cgroup, "user.slice", "user-"+id+".slice", "user@"+id+".service", rel, "cgroup.threads"), strings.Join(lines, "\n")+"\n")
}

func (f fake) task(tid int, uid uint32) {
	f.t.Helper()
	dir := filepath.Join(f.tree.Proc, strconv.Itoa(tid))
	f.write(filepath.Join(dir, "status"), "Name:\tworker\nUid:\t"+strconv.FormatUint(uint64(uid), 10)+"\t0\t0\t0\n")
	f.write(filepath.Join(dir, "timerslack_ns"), "50000\n")
}

func (f fake) slack(tid int) string {
	f.t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.tree.Proc, strconv.Itoa(tid), "timerslack_ns"))
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func TestEveryThreadOfTheUnitTakesTheSlackAndZeroRestoresIt(t *testing.T) {
	f := newFake(t)
	f.unit(user, "app.slice/app-chrome-10.scope", 10, 11)
	f.unit(user, "app.slice/app-chrome-10.scope/renderer", 12)
	f.unit(user, "app.slice/app-foot-20.scope", 20)
	for _, tid := range []int{10, 11, 12, 20} {
		f.task(tid, user)
	}
	if err := f.tree.SetTimerSlack(user, "app-chrome-10.scope", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for _, tid := range []int{10, 11, 12} {
		if got := f.slack(tid); got != "50000000" {
			t.Errorf("thread %d of the unit has slack %s, want 50 ms", tid, got)
		}
	}
	if got := f.slack(20); got != "50000" {
		t.Errorf("a thread of another unit had its slack set to %s", got)
	}
	if err := f.tree.SetTimerSlack(user, "app-chrome-10.scope", 0); err != nil {
		t.Fatal(err)
	}
	if got := f.slack(11); got != "0" {
		t.Errorf("restoring wrote %s, want 0, the kernel's word for the thread's default", got)
	}
}

func TestOnlyTheCallersOwnThreadsAndUnitsAreTouched(t *testing.T) {
	f := newFake(t)
	f.unit(user, "app.slice/app-term-30.scope", 30, 31)
	f.unit(1001, "app.slice/app-other-40.scope", 40)
	f.task(30, user)
	f.task(31, 0)
	f.task(40, 1001)
	if err := f.tree.SetTimerSlack(user, "app-term-30.scope", time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := f.slack(31); got != "50000" {
		t.Errorf("a root thread in the user's scope, as sudo is, got slack %s", got)
	}
	if err := f.tree.SetTimerSlack(user, "app-other-40.scope", time.Millisecond); !errors.Is(err, ErrNoUnit) {
		t.Errorf("another user's unit: %v, want %v", err, ErrNoUnit)
	}
	if got := f.slack(40); got != "50000" {
		t.Errorf("another user's thread got slack %s", got)
	}
}

func TestAThreadThatExitedIsSkipped(t *testing.T) {
	f := newFake(t)
	f.unit(user, "app.slice/app-a-50.scope", 50, 51)
	f.task(51, user)
	if err := f.tree.SetTimerSlack(user, nap.Unit("app-a-50.scope"), time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := f.slack(51); got != "1000000" {
		t.Errorf("the live thread has slack %s", got)
	}
}
