package main

import (
	"bufio"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

type sandboxMount struct {
	path     string
	writable bool
}

var unitKeysWithoutMounts = []string{
	"Type", "ExecStart", "Restart", "RestartSec",
	"NoNewPrivileges", "RestrictSUIDSGID", "LockPersonality",
}

var kernelTunables = []string{
	"/proc/sys", "/sys", "/proc/sysrq-trigger", "/proc/acpi", "/proc/fs",
	"/proc/irq", "/proc/latency_stats", "/proc/timer_stats",
}

func readServiceSection(t *testing.T, file string) map[string]string {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	keys := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if section != "[Service]" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("%s: %q is no key=value line", file, line)
		}
		if _, dup := keys[key]; dup {
			t.Fatalf("%s: %s is set twice, which the sandbox model does not follow", file, key)
		}
		keys[key] = value
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

func unitBool(t *testing.T, key, value string) bool {
	t.Helper()
	switch value {
	case "true", "yes":
		return true
	case "false", "no":
		return false
	}
	t.Fatalf("%s=%s is not a boolean the sandbox model reads", key, value)
	return false
}

func sandboxPaths(value string) []string {
	fields := strings.Fields(value)
	for i, field := range fields {
		fields[i] = strings.TrimLeft(field, "-+")
	}
	return fields
}

func sandboxOf(t *testing.T, file string) []sandboxMount {
	t.Helper()
	mounts := []sandboxMount{{path: "/", writable: true}}
	add := func(writable bool, paths ...string) {
		for _, p := range paths {
			mounts = append(mounts, sandboxMount{path: p, writable: writable})
		}
	}
	for key, value := range readServiceSection(t, file) {
		switch key {
		case "ProtectSystem":
			if value != "strict" {
				t.Fatalf("ProtectSystem=%s is not modelled", value)
			}
			add(false, "/")
			add(true, "/dev", "/proc", "/sys")
		case "ProtectKernelTunables":
			if unitBool(t, key, value) {
				add(false, kernelTunables...)
			}
		case "ProtectControlGroups":
			if unitBool(t, key, value) {
				add(false, "/sys/fs/cgroup")
			}
		case "ProtectHome":
			if unitBool(t, key, value) {
				add(false, "/home", "/root", "/run/user")
			}
		case "PrivateTmp":
			if unitBool(t, key, value) {
				add(true, "/tmp", "/var/tmp")
			}
		case "StateDirectory":
			for dir := range strings.FieldsSeq(value) {
				add(true, path.Join("/var/lib", dir))
			}
		case "ReadOnlyPaths":
			add(false, sandboxPaths(value)...)
		case "ReadWritePaths":
			add(true, sandboxPaths(value)...)
		default:
			if !slices.Contains(unitKeysWithoutMounts, key) {
				t.Fatalf("%s: the sandbox model does not know what %s does to the file system", file, key)
			}
		}
	}
	return mounts
}

func writableIn(mounts []sandboxMount, file string) bool {
	best, writable := -1, false
	for _, m := range mounts {
		inside := file == m.path || strings.HasPrefix(file, strings.TrimSuffix(m.path, "/")+"/")
		if !inside {
			continue
		}
		depth := strings.Count(strings.TrimSuffix(m.path, "/"), "/")
		if m.path == "/" {
			depth = 0
		}
		if depth > best || (depth == best && !m.writable) {
			best, writable = depth, m.writable
		}
	}
	return writable
}

func TestTheServiceCanWriteTheBatteryThroughItsDeviceTree(t *testing.T) {
	mounts := sandboxOf(t, "../../dist/lys-power.service")
	for _, file := range []string{
		"/sys/devices/LNXSYSTM:00/LNXSYBUS:00/PNP0C0A:00/power_supply/BAT0/charge_control_end_threshold",
		"/sys/devices/pci0000:00/0000:00:1f.0/PNP0C09:00/PNP0C0A:00/power_supply/BAT1/charge_control_start_threshold",
		"/run/battery-charge/full-now",
		"/var/lib/lys-power/unplugs",
	} {
		if !writableIn(mounts, file) {
			t.Errorf("lys-power.service leaves %s read-only", file)
		}
	}
}

func TestTheServiceKeepsKernelTunablesAndTheSystemReadOnly(t *testing.T) {
	mounts := sandboxOf(t, "../../dist/lys-power.service")
	for _, file := range []string{
		"/proc/sys/vm/swappiness",
		"/sys/kernel/mm/transparent_hugepage/enabled",
		"/sys/module/usbcore/parameters/autosuspend",
		"/sys/power/state",
		"/sys/firmware/acpi/interrupts/gpe00",
		"/sys/fs/cgroup/cgroup.procs",
		"/usr/bin/lys-power",
		"/etc/fstab",
		"/home/user/.profile",
	} {
		if writableIn(mounts, file) {
			t.Errorf("lys-power.service can write %s", file)
		}
	}
}
