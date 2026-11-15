package autobright

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/shell/service/brightness"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
	"github.com/stubbedev/lys/power/internal/camlight"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/lightsensor"
)

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Run is lys-power brightness: automatic brightness for the user's
// display session, on its session bus as autobrightness.ServiceName,
// until ctx ends.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) (err error) {
	light, interval, err := findLight(cfg, log)
	if err != nil {
		return err
	}
	p, err := params(interval)
	if err != nil {
		return err
	}

	system, err := dbusx.DialSystem(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closing("system bus", system.Close())) }()
	session, err := logind.NewManager(system).GetSession(ctx, logind.AutoSession)
	if err != nil {
		return fmt.Errorf("find the display session: %w", err)
	}
	backlight := brightness.NewSystem(system, false)
	defer func() { err = errors.Join(err, closing("backlight watch", backlight.Close())) }()

	sessionBus, err := dbusx.DialSession(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closing("session bus", sessionBus.Close())) }()

	d := New(light, backlight, session, realClock{}, log, p)
	server, release, err := autobrightness.Serve(sessionBus, autobrightness.Following, d.Resume)
	if err != nil {
		return err
	}
	defer release()
	return d.Run(ctx, server)
}

func closing(what string, err error) error {
	if err != nil {
		return fmt.Errorf("close the %s: %w", what, err)
	}
	return nil
}

// findLight prefers an ambient light sensor to the camera, and says how
// often the one found is sampled.
func findLight(cfg config.Config, log *slog.Logger) (Light, time.Duration, error) {
	sensor, err := lightsensor.Find(cfg.IIODir)
	if err == nil {
		log.Info("following the room with the ambient light sensor", "sensor", sensor.String())
		return sensor, config.SensorInterval, nil
	}
	if !errors.Is(err, lightsensor.ErrNone) {
		return nil, 0, err
	}
	camera, camErr := camlight.Find(cfg.DevDir, camlight.Params{
		SettleFrames:  config.CameraSettleFrames,
		DiscardFrames: config.CameraDiscardFrames,
		Brackets:      config.CameraBrackets,
		Frame:         config.CameraFrame,
		MaxExposure:   config.CameraMaxExposure,
		LumaLow:       config.CameraLumaLow,
		LumaTarget:    config.CameraLumaTarget,
		LumaHigh:      config.CameraLumaHigh,
		LuxScale:      config.CameraLuxScale,
	})
	if camErr != nil {
		return nil, 0, fmt.Errorf("nothing to measure the ambient light with: %w; %w", err, camErr)
	}
	log.Info("following the room with the camera", "camera", camera.String(), "every", config.CameraInterval)
	return camera, config.CameraInterval, nil
}

func params(interval time.Duration) (Params, error) {
	curve, err := ambient.NewCurve(config.DarkLux, config.BrightLux, config.BrightnessFloor, config.BrightnessCeiling)
	if err != nil {
		return Params{}, err
	}
	smoother, err := ambient.NewSmoother(config.SmoothingTau, config.MinLux)
	if err != nil {
		return Params{}, err
	}
	return Params{
		Curve:    curve,
		Smoother: smoother,
		Band:     config.HysteresisPercent,
		Steps:    config.TransitionSteps,
		Step:     config.TransitionStep,
		Interval: interval,
	}, nil
}
