package main

import (
	"context"
	"errors"
	"fmt"
	"math"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/shell/internal/cli"
	"github.com/stubbedev/lys/shell/service/brightness"
)

// brightnessAuto is the CHANGE that hands the backlight back to
// automatic brightness.
const brightnessAuto = "auto"

// brightnessRequest is a parsed CHANGE: a level for the shell's daemon
// to set, or automatic brightness resumed.
type brightnessRequest interface {
	run(m *cli.Matches) error
}

type setLevel struct{ change brightness.Change }

type resumeAuto struct{}

func parseBrightnessRequest(raw string) (brightnessRequest, error) {
	if raw == brightnessAuto {
		return resumeAuto{}, nil
	}
	change, err := brightness.ParseChange(raw)
	if err != nil {
		return nil, err
	}
	return setLevel{change}, nil
}

func brightnessCommand() *cli.Command {
	return &cli.Command{
		Name:  "brightness",
		About: "Set an output's brightness through its backlight or DDC/CI",
		Args: []*cli.Arg{
			{
				ID: "change", ValueName: "CHANGE", Required: true, AllowHyphenValues: true,
				Value: cli.Func(parseBrightnessRequest),
				Help:  "Percent: N sets, +N raises, -N lowers; never below modules.brightness.min-brightness; " + brightnessAuto + " hands the backlight back to automatic brightness",
			},
			{ID: "output", Short: 'o', Long: "output", ValueName: "NAME", Value: cli.String, Help: "Output connector name (e.g. DP-1); the focused output when omitted"},
		},
		Run: func(m *cli.Matches) error {
			request, _ := cli.Value[brightnessRequest](m, "change")
			return request.run(m)
		},
	}
}

func (s setLevel) run(m *cli.Matches) error {
	return withDaemon("Brightness", brightness.ServiceName, brightness.ServicePath, brightness.ServiceName, func(m *cli.Matches, p *daemonProxy) error {
		output, _ := cli.Value[string](m, "output")
		var name string
		var level float64
		if err := p.call("set brightness", "Apply", []any{&name, &level}, output, s.change.IsRelative(), s.change.Percent()); err != nil {
			return err
		}
		fmt.Fprintf(m.Stdout(), "%s: %d%%\n", name, int64(math.Round(level)))
		return nil
	})(m)
}

func (resumeAuto) run(m *cli.Matches) error {
	if output, ok := cli.Value[string](m, "output"); ok {
		return cliMessage("automatic brightness takes no output; drop --output " + output)
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return cliMessage("Failed to connect to D-Bus session bus: " + err.Error())
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	err = brightness.ResumeAuto(ctx, conn)
	switch {
	case errors.Is(err, brightness.ErrAutoNotRunning):
		return cliMessage(brightness.ErrAutoNotRunning.Error())
	case err != nil:
		return cliMessage("Failed to resume automatic brightness: " + err.Error())
	}
	return nil
}
