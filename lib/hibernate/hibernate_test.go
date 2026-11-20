package hibernate

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const (
	meminfo16G = "MemTotal:       16008812 kB\nMemFree:         1200000 kB\n"
	swapsNone  = "Filename\t\t\t\tType\t\tSize\t\tUsed\t\tPriority\n"
	swaps20G   = swapsNone + "/dev/nvme0n1p3                          partition\t20971516\t0\t\t-2\n"
	swaps8G    = swapsNone + "/dev/zram0                              partition\t8388604\t\t0\t\t100\n"
)

func root(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, data := range map[string]string{
		statePath:   "freeze mem disk\n",
		meminfoPath: meminfo16G,
		swapsPath:   swaps20G,
		cmdlinePath: "initrd=\\initrd.efi root=/dev/nvme0n1p2 resume=/dev/nvme0n1p3 rw\n",
	} {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(data)}
	}
	return fsys
}

func TestBlockersNamesEveryReasonTheSystemCannotHibernate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []Blocker
	}{
		{"swap larger than memory, resume set, no lockdown", map[string]string{lockdownPath: "[none] integrity confidentiality\n"}, nil},
		{"no securityfs lockdown file", nil, nil},
		{"lockdown", map[string]string{lockdownPath: "none [integrity] confidentiality\n"}, []Blocker{Lockdown}},
		{"no swap", map[string]string{swapsPath: swapsNone}, []Blocker{NoSwap}},
		{"swap smaller than memory", map[string]string{swapsPath: swaps8G}, []Blocker{SmallSwap}},
		{"no resume device", map[string]string{cmdlinePath: "root=/dev/sda2 quiet\n"}, []Blocker{NoResumeDevice}},
		{"kernel without hibernation", map[string]string{statePath: "freeze mem\n"}, []Blocker{KernelUnsupported}},
		{"everything", map[string]string{statePath: "freeze\n", lockdownPath: "none integrity [confidentiality]\n", swapsPath: swapsNone, cmdlinePath: "quiet\n"}, []Blocker{KernelUnsupported, Lockdown, NoSwap, NoResumeDevice}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Blockers(root(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("Blockers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBlockersFailsOnWhatItCannotReadInsteadOfGuessing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fsys  fstest.MapFS
		error string
	}{
		{"no meminfo", func() fstest.MapFS { f := root(nil); delete(f, meminfoPath); return f }(), "/proc/meminfo"},
		{"no MemTotal", root(map[string]string{meminfoPath: "MemFree: 1 kB\n"}), "no MemTotal"},
		{"swap size not a number", root(map[string]string{swapsPath: swapsNone + "/swap file big 0 -2\n"}), `size "big"`},
		{"no lockdown mode selected", root(map[string]string{lockdownPath: "none integrity\n"}), "no mode selected"},
		{"no power state", func() fstest.MapFS { f := root(nil); delete(f, statePath); return f }(), "/sys/power/state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Blockers(tc.fsys)
			if err == nil || !strings.Contains(err.Error(), tc.error) {
				t.Errorf("Blockers = %v, %v; want an error naming %s", got, err, tc.error)
			}
		})
	}
}

func TestBlockerNames(t *testing.T) {
	for b, want := range map[Blocker]string{
		KernelUnsupported: "kernel-unsupported",
		Lockdown:          "lockdown",
		NoSwap:            "no-swap",
		SmallSwap:         "small-swap",
		NoResumeDevice:    "no-resume-device",
		0:                 "Blocker(0)",
		6:                 "Blocker(6)",
	} {
		if got := b.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", b, got, want)
		}
	}
}
