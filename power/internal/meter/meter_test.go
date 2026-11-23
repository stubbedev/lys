package meter_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/procstat"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/meter"
	"github.com/stubbedev/lys/power/internal/supply"
)

const (
	appScope = "user.slice/user-1000.slice/user@1000.service/app.slice/app-foot-7.scope"
	service  = "system.slice/lys-power.service"
	session  = "user.slice/user-1000.slice/session-2.scope"
)

var at = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

type tree struct {
	t    testing.TB
	root string
}

func (tr tree) write(rel, content string) {
	tr.t.Helper()
	path := filepath.Join(tr.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) link(rel, target string) {
	tr.t.Helper()
	path := filepath.Join(tr.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		tr.t.Fatal(err)
	}
}

type machine struct {
	m                      meter.Meter
	cgroup, proc, powercap tree
	supply, dri            tree
}

func newMachine(t testing.TB) machine {
	dir := t.TempDir()
	mc := machine{
		cgroup:   tree{t, filepath.Join(dir, "cgroup")},
		proc:     tree{t, filepath.Join(dir, "proc")},
		powercap: tree{t, filepath.Join(dir, "powercap")},
		supply:   tree{t, filepath.Join(dir, "supply")},
		dri:      tree{t, filepath.Join(dir, "dri")},
	}
	mc.m = meter.Meter{Cgroup: mc.cgroup.root, Proc: mc.proc.root, Powercap: mc.powercap.root, DRIDebug: mc.dri.root, Supply: supply.Supply{Dir: mc.supply.root}}
	mc.cgroup.write("cpu.stat", "usage_usec 9000000\nuser_usec 1\n")
	mc.cgroup.write("cgroup.procs", "")
	if err := os.MkdirAll(mc.supply.root, 0o755); err != nil {
		t.Fatal(err)
	}
	return mc
}

func (mc machine) unit(rel string, usec string, pids string) {
	mc.cgroup.write(rel+"/cpu.stat", "usage_usec "+usec+"\n")
	mc.cgroup.write(rel+"/cgroup.procs", pids)
}

func (mc machine) thread(pid, tid, name string, voluntary, involuntary string) {
	mc.proc.write(pid+"/task/"+tid+"/status", "Name:\t"+name+"\nState:\tS\nvoluntary_ctxt_switches:\t"+voluntary+"\nnonvoluntary_ctxt_switches:\t"+involuntary+"\n")
}

func (mc machine) process(pid, comm string) {
	mc.stat(pid, comm, "3", "2", "100")
	mc.proc.link(pid+"/exe", "/bin/"+comm)
	if err := os.MkdirAll(filepath.Join(mc.proc.root, pid, "fd"), 0o755); err != nil {
		mc.proc.t.Fatal(err)
	}
}

func (mc machine) stat(pid, comm, utime, stime, start string) {
	mc.statOf(pid, comm, "1", utime, stime, start)
}

func (mc machine) statOf(pid, comm, parent, utime, stime, start string) {
	mc.proc.write(pid+"/stat", pid+" ("+comm+") S "+parent+" "+pid+" "+pid+" 0 -1 4194560 100 0 0 0 "+utime+" "+stime+" 0 0 20 0 1 0 "+start+" 12345 67\n")
}

func pids(g energy.Group) []uint32 {
	out := make([]uint32, 0, len(g.Processes))
	for _, p := range g.Processes {
		out = append(out, p.ID.PID)
	}
	return out
}

func (mc machine) sample(t *testing.T) (energy.Sample, meter.Gaps) {
	t.Helper()
	s, gaps, err := mc.m.Sample(func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	return s, gaps
}

func find(t *testing.T, s energy.Sample, cgroup string) energy.Group {
	t.Helper()
	for _, g := range s.Groups {
		if g.Cgroup == cgroup {
			return g
		}
	}
	t.Fatalf("%s is not among %+v", cgroup, s.Groups)
	return energy.Group{}
}

func TestEveryUnitIsAnAppAndSlicesAreWalked(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "250", "")
	mc.unit(service, "1000", "")
	mc.unit(session, "5", "")
	mc.cgroup.write("user.slice/user-1000.slice/user@1000.service/cpu.stat", "usage_usec 99\n")
	mc.unit("user.slice/user-1000.slice/user@1000.service/init.scope", "1", "")
	mc.unit(service+"/payload", "0", "")
	s, _ := mc.sample(t)
	var got []string
	for _, g := range s.Groups {
		got = append(got, g.Cgroup)
	}
	want := []string{"system.slice/lys-power.service", session, "user.slice/user-1000.slice/user@1000.service/app.slice/app-foot-7.scope", "user.slice/user-1000.slice/user@1000.service/init.scope"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %q, want %q", got, want)
	}
	if g := find(t, s, appScope); g.CPU != 250*time.Microsecond {
		t.Errorf("CPU = %s, want 250µs", g.CPU)
	}
	if s.TotalCPU != 9*time.Second || !s.At.Equal(at) {
		t.Errorf("sample = %s at %s", s.TotalCPU, s.At)
	}
}

func TestProcessesThreadsAndTheirSwitches(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "41\n40\n")
	mc.cgroup.write(appScope+"/child/cgroup.procs", "42\n41\n")
	mc.process("40", "foot")
	mc.thread("40", "40", "foot", "10", "2")
	mc.thread("40", "43", "foot:gdrv0", "5", "0")
	mc.process("41", "fish")
	mc.thread("41", "41", "fish", "1", "1")
	mc.process("42", "sleep")
	mc.thread("42", "42", "sleep", "0", "0")
	s, _ := mc.sample(t)
	g := find(t, s, appScope)
	if got := pids(g); !reflect.DeepEqual(got, []uint32{40, 41, 42}) {
		t.Errorf("pids %v, want 40 41 42", got)
	}
	want := map[string][]energy.Thread{
		"foot":  {{TID: 40, Name: "foot", Switches: 12}, {TID: 43, Name: "foot:gdrv0", Switches: 5}},
		"fish":  {{TID: 41, Name: "fish", Switches: 2}},
		"sleep": {{TID: 42, Name: "sleep", Switches: 0}},
	}
	for _, p := range g.Processes {
		if !reflect.DeepEqual(p.Threads, want[p.Command]) {
			t.Errorf("%d %s threads = %+v, want %+v", p.ID.PID, p.Command, p.Threads, want[p.Command])
		}
	}
}

func TestEachProcessCarriesItsCPUTimeAndStart(t *testing.T) {
	mc := newMachine(t)
	mc.unit(session, "1", "7356\n7826\n")
	mc.statOf("7356", ".lys-login-wrap", "1701", "1", "1", "500")
	mc.proc.write("7356/cmdline", "/nix/store/ks59-lys-login/bin/lys-login\x00worker\x00session\x00")
	mc.proc.link("7356/exe", "/nix/store/ks59-lys-login/bin/.lys-login-wrapped")
	mc.thread("7356", "7356", ".lys-login-wrap", "0", "0")
	mc.statOf("7826", "lys-wm", "7356", "15000", "8367", "900")
	mc.proc.link("7826/exe", "/nix/store/3caj-lys-wm/bin/lys-wm")
	mc.thread("7826", "7826", "lys-wm", "0", "0")
	s, _ := mc.sample(t)
	g := find(t, s, session)
	worker, err := procstat.Stat{CPUTicks: 2}.CPU()
	if err != nil {
		t.Fatal(err)
	}
	wm, err := procstat.Stat{CPUTicks: 23367}.CPU()
	if err != nil {
		t.Fatal(err)
	}
	want := []energy.Process{
		{
			ID: energy.ProcessID{PID: 7356, Start: 500}, Parent: 1701, Command: "lys-login", Exe: "/nix/store/ks59-lys-login/bin/.lys-login-wrapped",
			CPU: worker, Threads: []energy.Thread{{TID: 7356, Name: ".lys-login-wrap"}}, GPU: map[energy.DRMClient]map[string]energy.Engine{},
		},
		{
			ID: energy.ProcessID{PID: 7826, Start: 900}, Parent: 7356, Command: "lys-wm", Exe: "/nix/store/3caj-lys-wm/bin/lys-wm",
			CPU: wm, Threads: []energy.Thread{{TID: 7826, Name: "lys-wm"}}, GPU: map[energy.DRMClient]map[string]energy.Engine{},
		},
	}
	if !reflect.DeepEqual(g.Processes, want) {
		t.Errorf("processes = %+v, want %+v", g.Processes, want)
	}
}

func TestAProcessIsNamedForItsProgramWhereItsCommandNameIsShortened(t *testing.T) {
	for _, tc := range []struct {
		comm, cmdline, want string
	}{
		{".lys-login-wrap", "/nix/store/x-lys-login/bin/lys-login\x00daemon\x00", "lys-login"},
		{".claude-unwrapp", "/nix/store/x-claude-code/bin/claude\x00", "claude"},
		{".gh-wrapped", "/nix/store/x-gh/bin/gh\x00pr\x00", "gh"},
		{".xdg-desktop-po", "/nix/store/x-portal/libexec/xdg-desktop-portal\x00", "xdg-desktop-portal"},
		{"systemd-journal", "/nix/store/x-systemd/lib/systemd/systemd-journald\x00", "systemd-journald"},
		{"tmux: server", "/nix/store/x-tmux/bin/tmux\x00", "tmux: server"},
		{"oom-notify", "/nix/store/x-bash/bin/bash\x00/etc/oom-notify\x00", "oom-notify"},
		{"tmux-pick-sessi", "/nix/store/x-zsh/bin/zsh\x00/home/u/bin/tmux-pick-session\x00", "tmux-pick-sessi"},
		{".hidden", "/opt/other\x00", ".hidden"},
		{".kworker-wrapp", "", ".kworker-wrapp"},
	} {
		mc := newMachine(t)
		mc.unit(appScope, "1", "40\n")
		mc.stat("40", tc.comm, "0", "0", "1")
		mc.proc.link("40/exe", "/bin/x")
		mc.proc.write("40/cmdline", tc.cmdline)
		mc.thread("40", "40", tc.comm, "0", "0")
		s, _ := mc.sample(t)
		if got := find(t, s, appScope).Processes[0].Command; got != tc.want {
			t.Errorf("%q run as %q is named %q, want %q", tc.comm, tc.cmdline, got, tc.want)
		}
	}
}

func TestACommandLineLongerThanAPathStillNamesTheProgram(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n")
	mc.stat("40", ".java-wrapped", "0", "0", "1")
	mc.proc.link("40/exe", "/bin/java")
	mc.proc.write("40/cmdline", "/usr/bin/java\x00-cp\x00"+strings.Repeat("x", 3*4096)+"\x00")
	mc.thread("40", "40", "java", "0", "0")
	s, _ := mc.sample(t)
	if got := find(t, s, appScope).Processes[0].Command; got != "java" {
		t.Errorf("named %q, want java", got)
	}
}

func TestAProcessWhoseProgramIsGoneIsSkipped(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n41\n")
	mc.process("40", "foot")
	mc.thread("40", "40", "foot", "0", "0")
	mc.stat("41", "zombie", "0", "0", "1")
	mc.thread("41", "41", "zombie", "0", "0")
	s, _ := mc.sample(t)
	if got := pids(find(t, s, appScope)); !reflect.DeepEqual(got, []uint32{40}) {
		t.Errorf("pids = %v, want 40 alone: 41 has no program left", got)
	}
}

func TestAMalformedProcessStatIsAnError(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n")
	mc.proc.write("40/stat", "40 (foot) S 1 2 3\n")
	mc.thread("40", "40", "foot", "0", "0")
	if _, _, err := mc.m.Sample(time.Now); err == nil {
		t.Error("a truncated /proc/40/stat sampled")
	}
}

func TestAProcessThatExitsMidSampleIsSkipped(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n50\n")
	mc.process("40", "foot")
	mc.thread("40", "40", "foot", "1", "1")
	mc.process("50", "gone")
	s, _ := mc.sample(t)
	if got := pids(find(t, s, appScope)); !reflect.DeepEqual(got, []uint32{40}) {
		t.Errorf("pids = %v, want 40 alone: 50 has no threads left", got)
	}
	mc.unit(appScope, "1", "40\n60\n")
	s, _ = mc.sample(t)
	if got := pids(find(t, s, appScope)); !reflect.DeepEqual(got, []uint32{40}) {
		t.Errorf("pids = %v, want 40 alone: 60 has no stat left", got)
	}
}

func TestAMalformedThreadStatusIsAnError(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n")
	mc.process("40", "foot")
	mc.proc.write("40/task/40/status", "Name:\tfoot\n")
	if _, _, err := mc.m.Sample(time.Now); err == nil {
		t.Error("a status without switch counts sampled")
	}
}

func TestACgroupWithoutUsageIsAnError(t *testing.T) {
	mc := newMachine(t)
	mc.cgroup.write("cpu.stat", "user_usec 1\n")
	if _, _, err := mc.m.Sample(time.Now); err == nil {
		t.Error("a root without usage_usec sampled")
	}
}

func TestGPUClientsAreReadOnceAcrossProcesses(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n41\n")
	info := "pos:\t0\nflags:\t02100002\ndrm-driver:\tamdgpu\ndrm-pdev:\t0000:03:00.0\ndrm-client-id:\t7\ndrm-engine-gfx:\t1500 ns\ndrm-engine-capacity-gfx:\t1\n"
	for _, pid := range []string{"40", "41"} {
		mc.process(pid, "app")
		mc.thread(pid, pid, "app", "0", "0")
		mc.proc.link(pid+"/fd/9", "/dev/dri/renderD128")
		mc.proc.write(pid+"/fdinfo/9", info)
		mc.proc.link(pid+"/fd/1", "/dev/null")
		mc.proc.write(pid+"/fdinfo/1", "pos:\t0\n")
	}
	mc.proc.link("40/fd/10", "/dev/dri/renderD128")
	mc.proc.write("40/fdinfo/10", "drm-driver:\ti915\ndrm-client-id:\t3\ndrm-engine-render:\t20 ns\n")
	s, _ := mc.sample(t)
	want := map[energy.DRMClient]map[string]energy.Engine{
		{Device: "0000:03:00.0", ID: 7}: {"gfx": {Clock: energy.EngineNanoseconds, Busy: 1500}},
		{Device: "i915", ID: 3}:         {"render": {Clock: energy.EngineNanoseconds, Busy: 20}},
	}
	g := find(t, s, appScope)
	if !reflect.DeepEqual(g.Processes[0].GPU, want) || len(g.Processes[1].GPU) != 0 {
		t.Errorf("GPU = %+v and %+v, want %+v on 40 and the shared client not again on 41", g.Processes[0].GPU, g.Processes[1].GPU, want)
	}
}

func TestOnlyDescriptorsOnADRMDeviceAreGPUClients(t *testing.T) {
	mc := newMachine(t)
	mc.unit(appScope, "1", "40\n")
	mc.process("40", "app")
	mc.thread("40", "40", "app", "0", "0")
	info := "drm-driver:\ti915\ndrm-client-id:\t%s\ndrm-engine-render:\t20 ns\n"
	for fd, target := range map[string]string{"3": "/dev/dri", "4": "/dev/drink/x", "5": "/dev/dr", "6": "/dev/dri/card0"} {
		mc.proc.link("40/fd/"+fd, target)
		mc.proc.write("40/fdinfo/"+fd, fmt.Sprintf(info, fd))
	}
	s, _ := mc.sample(t)
	want := map[energy.DRMClient]map[string]energy.Engine{
		{Device: "i915", ID: 6}: {"render": {Clock: energy.EngineNanoseconds, Busy: 20}},
	}
	if got := find(t, s, appScope).Processes[0].GPU; !reflect.DeepEqual(got, want) {
		t.Errorf("GPU = %+v, want only the descriptor under /dev/dri/: %+v", got, want)
	}
}

func TestParseFDInfo(t *testing.T) {
	xe := []byte("drm-driver:\txe\ndrm-pdev:\t0000:00:02.0\ndrm-client-id:\t12\ndrm-cycles-rcs:\t400\ndrm-total-cycles-rcs:\t9000\ndrm-cycles-bcs:\t0\ndrm-total-cycles-bcs:\t9000\n")
	client, engines, ok, err := meter.ParseFDInfo(xe)
	if err != nil || !ok {
		t.Fatalf("xe: %v %v", ok, err)
	}
	if client != (energy.DRMClient{Device: "0000:00:02.0", ID: 12}) {
		t.Errorf("client = %+v", client)
	}
	want := map[string]energy.Engine{
		"rcs": {Clock: energy.EngineCycles, Busy: 400, Total: 9000},
		"bcs": {Clock: energy.EngineCycles, Busy: 0, Total: 9000},
	}
	if !reflect.DeepEqual(engines, want) {
		t.Errorf("engines = %+v, want %+v", engines, want)
	}

	if _, _, ok, err := meter.ParseFDInfo([]byte("pos:\t0\nflags:\t0\nmnt_id:\t5\n")); ok || err != nil {
		t.Errorf("a file that is not a DRM client: %v %v", ok, err)
	}
	for _, bad := range []string{
		"drm-client-id:\tseven\n",
		"drm-client-id:\t1\ndrm-engine-gfx:\tfast ns\n",
		"drm-client-id:\t1\ndrm-cycles-rcs:\t-1\n",
	} {
		if _, _, _, err := meter.ParseFDInfo([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestEnergyCountersPreferThePlatformZone(t *testing.T) {
	mc := newMachine(t)
	zone := func(name, label, uj string) {
		mc.powercap.write(name+"/name", label+"\n")
		mc.powercap.write(name+"/energy_uj", uj+"\n")
		mc.powercap.write(name+"/max_energy_range_uj", "262143328850\n")
	}
	zone("intel-rapl:0", "package-0", "100")
	zone("intel-rapl:0:0", "core", "50")
	zone("intel-rapl:1", "package-1", "200")
	s, _ := mc.sample(t)
	want := []energy.Counter{{Microjoules: 100, Range: 262143328850}, {Microjoules: 200, Range: 262143328850}}
	if !reflect.DeepEqual(s.Energy, want) {
		t.Errorf("packages = %+v, want %+v", s.Energy, want)
	}
	zone("intel-rapl:2", "psys", "300")
	s, _ = mc.sample(t)
	if want := []energy.Counter{{Microjoules: 300, Range: 262143328850}}; !reflect.DeepEqual(s.Energy, want) {
		t.Errorf("with psys = %+v, want %+v", s.Energy, want)
	}
}

func TestUnreadableEnergyIsAGapNotAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a file without read permission")
	}
	mc := newMachine(t)
	mc.powercap.write("intel-rapl:0/name", "package-0\n")
	mc.powercap.write("intel-rapl:0/energy_uj", "1\n")
	mc.powercap.write("intel-rapl:0/max_energy_range_uj", "10\n")
	if err := os.Chmod(filepath.Join(mc.powercap.root, "intel-rapl:0/energy_uj"), 0); err != nil {
		t.Fatal(err)
	}
	s, gaps := mc.sample(t)
	if s.Energy != nil || !gaps.EnergyUnreadable {
		t.Errorf("energy %+v, gaps %+v: want none and the gap named", s.Energy, gaps)
	}
}

func TestTheBatteryCountsOnlyWhileDischarging(t *testing.T) {
	mc := newMachine(t)
	mc.supply.write("BAT0/type", "Battery\n")
	mc.supply.write("BAT0/status", "Discharging\n")
	mc.supply.write("BAT0/power_now", "16500000\n")
	s, _ := mc.sample(t)
	if s.Battery == nil || *s.Battery != 16500000 {
		t.Errorf("battery = %v, want 16.5 W", s.Battery)
	}
	mc.supply.write("BAT0/status", "Charging\n")
	if s, _ = mc.sample(t); s.Battery != nil {
		t.Errorf("charging, battery = %v, want none", *s.Battery)
	}
	if err := os.Remove(filepath.Join(mc.supply.root, "BAT0/power_now")); err != nil {
		t.Fatal(err)
	}
	mc.supply.write("BAT0/status", "Discharging\n")
	mc.supply.write("BAT0/current_now", "-2000000\n")
	mc.supply.write("BAT0/voltage_now", "12000000\n")
	if s, _ = mc.sample(t); s.Battery == nil || *s.Battery != 24000000 {
		t.Errorf("from current and voltage, battery = %v, want 24 W", s.Battery)
	}
	if err := os.Remove(filepath.Join(mc.supply.root, "BAT0/voltage_now")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := mc.m.Sample(time.Now); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a battery reporting no power: %v, want the missing file named", err)
	}
}

func BenchmarkSampleProcessesWithGPUClients(b *testing.B) {
	mc := newMachine(b)
	var unit []string
	for p := range 40 {
		pid := strconv.Itoa(100 + p)
		unit = append(unit, pid)
		mc.process(pid, "app")
		mc.thread(pid, pid, "app", "10", "2")
		for fd := range 40 {
			mc.proc.link(pid+"/fd/"+strconv.Itoa(fd), "/dev/null")
			mc.proc.write(pid+"/fdinfo/"+strconv.Itoa(fd), "pos:\t0\n")
		}
		mc.proc.link(pid+"/fd/40", "/dev/dri/renderD128")
		mc.proc.write(pid+"/fdinfo/40", "drm-driver:\ti915\ndrm-client-id:\t"+pid+"\ndrm-engine-render:\t20 ns\ndrm-engine-video:\t5 ns\n")
	}
	mc.unit("app.slice/app.scope", "500", strings.Join(unit, "\n")+"\n")
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := mc.m.Sample(func() time.Time { return at }); err != nil {
			b.Fatal(err)
		}
	}
}
