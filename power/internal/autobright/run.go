package autobright

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight"
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"

	"github.com/stubbedev/lys/power/autobrightness"
	"github.com/stubbedev/lys/power/internal/ambient"
	"github.com/stubbedev/lys/power/internal/camlight"
	"github.com/stubbedev/lys/power/internal/closing"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/lightsensor"
)

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Run is lys-power brightness: automatic brightness for the user's
// display session, on its session bus as autobrightness.ServiceName,
// until ctx ends.
func Run(ctx context.Context, cfg config.Paths, log *slog.Logger) (err error) {
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
	defer closing.Join(&err, "system bus", system)
	backlight := backlight.NewSysfs(system)
	defer closing.Join(&err, "backlight watch", backlight)

	sessionBus, err := dbusx.DialSession(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer closing.Join(&err, "session bus", sessionBus)

	d := New(light, backlight, displaySessions{logind.NewManager(system)}, lysProfile{source: powerprofiles.NewOn(system), log: log}, realClock{}, log, p)
	server, release, err := autobrightness.Serve(sessionBus, autobrightness.Following, d.Resume)
	if err != nil {
		return err
	}
	defer release()
	return d.Run(ctx, server)
}

type displaySessions struct{ manager logind.Manager }

func (s displaySessions) Display(ctx context.Context) (Session, error) {
	session, err := s.manager.GetSession(ctx, logind.AutoSession)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// lysProfile is the active profile lys-power publishes.
type lysProfile struct {
	source powerprofiles.Source
	log    *slog.Logger
}

func (p lysProfile) Saver(ctx context.Context) (<-chan bool, error) {
	return powerprofiles.FollowSaver(ctx, p.source, config.BusTimeout, func(err error) {
		p.log.Error("following the power profile", "err", err)
	})
}

// findLight prefers an ambient light sensor to the camera, and says how
// often the one found is sampled.
func findLight(cfg config.Paths, log *slog.Logger) (Light, time.Duration, error) {
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
	dim, err := ambient.NewDim(config.LowPowerShare)
	if err != nil {
		return Params{}, err
	}
	room, err := ambient.NewRoomBand(config.RoomDimLux, config.RoomBrightLux)
	if err != nil {
		return Params{}, err
	}
	return Params{
		Curve:    curve,
		LowPower: dim,
		Room:     room,
		Smoother: smoother,
		Band:     config.HysteresisPercent,
		Steps:    config.TransitionSteps,
		Step:     config.TransitionStep,
		Interval: interval,
	}, nil
}
