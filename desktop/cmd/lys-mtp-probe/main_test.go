package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/mtp/usb"
)

const (
	sysfs       = "../../mtp/usb/testdata/devices"
	rulesFile   = "69-lys-mtp.rules"
	installPath = "/usr/lib/udev/lys-mtp-probe"
)

type probeCall struct {
	answer bool
	err    error
	asked  []string
}

func (p *probeCall) probe(_ context.Context, d usb.Device) (bool, error) {
	p.asked = append(p.asked, d.Name)
	return p.answer, p.err
}

func probeRun(t *testing.T, p *probeCall, args ...string) (status int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status = run(args, sysfs, p.probe, &out, &errOut)
	return status, out.String(), errOut.String()
}

func TestProbeTagsCamerasAndPhones(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, device string
		probe        probeCall
		want         string
		asked        []string
	}{
		{"PTP camera", "1-2", probeCall{}, "ID_LYS_USB_PROTOCOL=PTP\n", nil},
		{"phone naming its interface MTP", "1-3", probeCall{}, "ID_LYS_USB_PROTOCOL=MTP\nID_MTP_DEVICE=1\n", nil},
		{"phone naming its still image interface MTP", "2-1", probeCall{}, "ID_LYS_USB_PROTOCOL=MTP\nID_MTP_DEVICE=1\n", nil},
		{"vendor interface whose OS descriptor says MTP", "1-5", probeCall{answer: true}, "ID_LYS_USB_PROTOCOL=MTP\nID_MTP_DEVICE=1\n", []string{"1-5"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			status, stdout, stderr := probeRun(t, &c.probe, c.device)
			if status != 0 || stdout != c.want || stderr != "" {
				t.Fatalf("probe of %s exited %d printing %q, %q; want 0 printing %q", c.device, status, stdout, stderr, c.want)
			}
			if strings.Join(c.probe.asked, " ") != strings.Join(c.asked, " ") {
				t.Fatalf("probe of %s asked the OS descriptors of %v, want %v", c.device, c.probe.asked, c.asked)
			}
		})
	}
}

func TestProbeLeavesOtherDevicesUntagged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, device string
		probe        probeCall
		asked        bool
	}{
		{"keyboard", "1-4", probeCall{answer: true}, false},
		{"hub interface", "1-0", probeCall{answer: true}, false},
		{"root hub", "usb1", probeCall{answer: true}, false},
		{"device unplugged", "9-9", probeCall{answer: true}, false},
		{"vendor interface that is not MTP", "1-5", probeCall{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			status, stdout, stderr := probeRun(t, &c.probe, c.device)
			if status != 0 || stdout != "" || stderr != "" {
				t.Fatalf("probe of %s exited %d printing %q, %q; want 0 printing nothing", c.device, status, stdout, stderr)
			}
			if asked := len(c.probe.asked) > 0; asked != c.asked {
				t.Fatalf("probe of %s asked the OS descriptors: %t, want %t", c.device, asked, c.asked)
			}
		})
	}
}

func TestProbeFailsWhenTheDescriptorsCannotBeRead(t *testing.T) {
	t.Parallel()
	p := probeCall{err: errors.New("usb: probe the OS descriptors of /dev/bus/usb/001/011: timed out")}
	status, stdout, stderr := probeRun(t, &p, "1-5")
	if status != exitFailed || stdout != "" || !strings.Contains(stderr, "1-5") || !strings.Contains(stderr, "timed out") {
		t.Fatalf("probe failing exited %d printing %q, %q; want %d naming the device and the error", status, stdout, stderr, exitFailed)
	}
}

func TestProbeFailsOnAnUnreadableSysfs(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	status := run([]string{"1-2"}, t.TempDir()+"/none", (&probeCall{}).probe, &out, &errOut)
	if status != exitFailed || out.Len() != 0 || !strings.Contains(errOut.String(), "none") {
		t.Fatalf("probe of a missing sysfs exited %d printing %q, %q", status, out.String(), errOut.String())
	}
}

func TestProbeTakesOneDevice(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"1-2", "1-3"}} {
		status, stdout, stderr := probeRun(t, &probeCall{}, args...)
		if status != exitUsage || stdout != "" || !strings.Contains(stderr, "usage:") {
			t.Errorf("probe of %q exited %d printing %q, %q; want %d and the usage", args, status, stdout, stderr, exitUsage)
		}
	}
}

func TestRuleRunsTheInstalledProbeAndTagsOnlyWhatItNames(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	rules := string(b)
	for _, line := range []string{
		`ENV{DEVTYPE}!="usb_device", GOTO="lys_mtp_end"`,
		`IMPORT{program}="` + installPath + ` %k"`,
		`ENV{` + protocolKey + `}=="` + usb.PTP.String() + `|` + usb.MTP.String() + `", TAG+="uaccess"`,
		`LABEL="lys_mtp_end"`,
	} {
		if !strings.Contains(rules, line+"\n") {
			t.Errorf("%s has no line %s:\n%s", rulesFile, line, rules)
		}
	}
	if n := strings.Count(rules, `TAG+=`); n != 1 {
		t.Errorf("%s tags devices on %d lines, want only the probed one:\n%s", rulesFile, n, rules)
	}
	if i, j := strings.Index(rules, "IMPORT{program}"), strings.Index(rules, "TAG+="); i < 0 || j < i {
		t.Errorf("%s tags before it runs the probe:\n%s", rulesFile, rules)
	}
}
