package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/energyd"
	"github.com/stubbedev/lys/power/internal/meter"
)

type energyOptions struct {
	over    time.Duration
	threads bool
	json    bool
}

func parseEnergy(args []string) (energyOptions, error) {
	fs := flag.NewFlagSet("energy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var o energyOptions
	fs.DurationVar(&o.over, "for", 10*time.Second, "")
	fs.BoolVar(&o.threads, "threads", false, "")
	fs.BoolVar(&o.json, "json", false, "")
	if err := fs.Parse(args); err != nil {
		return o, fmt.Errorf("%w: energy %w\n\n%s", errUnknownCommand, err, usage)
	}
	if fs.NArg() > 0 {
		return o, fmt.Errorf("%w: energy takes no argument %q\n\n%s", errUnknownCommand, fs.Arg(0), usage)
	}
	if o.over <= 0 {
		return o, fmt.Errorf("%w: energy -for %s: want a positive duration\n\n%s", errUnknownCommand, o.over, usage)
	}
	return o, nil
}

type energyMeter interface {
	energyd.Sampler
	Panels() ([]energy.PanelState, bool, error)
}

func measureEnergy(m energyMeter, over time.Duration, sleep func(time.Duration)) (energy.Report, meter.Gaps, error) {
	first, _, err := m.Sample(time.Now)
	if err != nil {
		return energy.Report{}, meter.Gaps{}, err
	}
	readings := first.Panels
	for left := over; left > 0; {
		step := min(energy.PanelPoll, left)
		sleep(step)
		left -= step
		if left == 0 {
			break
		}
		panels, _, err := m.Panels()
		if err != nil {
			return energy.Report{}, meter.Gaps{}, err
		}
		readings = append(readings, panels...)
	}
	second, gaps, err := m.Sample(time.Now)
	if err != nil {
		return energy.Report{}, gaps, err
	}
	iv, err := energy.Measure(first, second)
	if err != nil {
		return energy.Report{}, gaps, err
	}
	iv.Panels = append(readings, iv.Panels...)
	return energy.Attribute([]energy.Interval{iv}), gaps, nil
}

func writeEnergy(w io.Writer, r energy.Report, gaps meter.Gaps, o energyOptions) error {
	if o.json {
		return json.NewEncoder(w).Encode(r)
	}
	if r.Source == energy.SourceNone {
		fmt.Fprintf(w, "power:  not measured over %s\n", r.Covered.Round(time.Millisecond))
	} else {
		fmt.Fprintf(w, "power:  %s from the %s over %s\n", watts(r.Watts), r.Source, r.Covered.Round(time.Millisecond))
	}
	if gaps.EnergyUnreadable {
		fmt.Fprintln(w, "        the RAPL energy counters are root's to read")
	}
	if gaps.GPUUnreadable > 0 {
		fmt.Fprintf(w, "        GPU time of %d processes is unreadable without root\n", gaps.GPUUnreadable)
	}
	for _, p := range r.Panels {
		fmt.Fprintf(w, "panel:  %s %s, self-refreshing in %s of %d readings\n", p.Device, p.Mode, percent(p.Residency), p.Readings)
	}
	if gaps.PanelsUnreadable {
		fmt.Fprintln(w, "        panel self refresh in the DRM debugfs is root's to read")
	}
	if len(r.Apps) == 0 {
		fmt.Fprintln(w, "\nno app was busy")
		return nil
	}
	fmt.Fprintln(w)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COMMAND\tUNIT\tWATTS\tCPU\tWAKEUPS/S\tGPU\tIMPACT")
	for _, a := range r.Apps {
		power := "-"
		if r.Source != energy.SourceNone {
			power = watts(a.Watts)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", a.Command, a.Unit(), power, percent(a.CPU), rate(a.Wakeups), percent(a.GPU), percent(a.Impact))
		if !o.threads {
			continue
		}
		for _, th := range a.Threads {
			fmt.Fprintf(tw, "  %s/%s\t%d/%d\t\t\t%s\n", th.Process, th.Name, th.PID, th.TID, rate(th.Wakeups))
		}
	}
	return tw.Flush()
}

func watts(w float64) string { return strconv.FormatFloat(w, 'f', 2, 64) + " W" }

func percent(f float64) string { return strconv.FormatFloat(f*100, 'f', 1, 64) + "%" }

func rate(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }
