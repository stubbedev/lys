package attend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/attention"
	"github.com/stubbedev/lys/power/internal/closing"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/facecam"
	"github.com/stubbedev/lys/power/internal/presence"
)

func Run(ctx context.Context, cfg config.Paths, log *slog.Logger) (err error) {
	cameras, err := FindCameras(cfg.DevDir, facecam.V4L2{SysDir: cfg.SysDir})
	if err != nil && !errors.Is(err, facecam.ErrNone) {
		return err
	}
	sessionBus, err := dbusx.DialSession(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer closing.Join(&err, "session bus", sessionBus)
	if len(cameras) == 0 {
		return serve(ctx, sessionBus, noCamera(cfg.DevDir), log, "answering attention checks without a camera", "in", cfg.DevDir)
	}
	system, err := dbusx.DialSystem(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer closing.Join(&err, "system bus", system)
	manager := logind.NewManager(system)
	display := func(ctx context.Context) (Session, error) {
		session, err := manager.GetSession(ctx, logind.AutoSession)
		if err != nil {
			return nil, err
		}
		return session, nil
	}
	return serve(ctx, sessionBus, New(display, cameras, log).Check, log, "answering attention checks", "cameras", fmt.Sprint(cameras))
}

func serve(ctx context.Context, bus *dbus.Conn, check attention.Check, log *slog.Logger, msg string, args ...any) error {
	release, err := attention.Serve(bus, check)
	if err != nil {
		return err
	}
	defer release()
	log.Info(msg, args...)
	<-ctx.Done()
	return nil
}

func noCamera(dir string) attention.Check {
	return func(context.Context) (idlepolicy.Presence, error) {
		return 0, fmt.Errorf("%w in %s", attention.ErrNoCamera, dir)
	}
}

func FindCameras(dir string, devices facecam.Devices) ([]Camera, error) {
	judge, err := presence.NewFaceJudge()
	if err != nil {
		return nil, err
	}
	found, err := facecam.Find(dir, devices, judge, facecam.Params{
		SettleFrames: config.AttentionSettleFrames,
		Frames:       config.AttentionFrames,
		Frame:        config.AttentionFrame,
	})
	if err != nil {
		return nil, err
	}
	cameras := make([]Camera, len(found))
	for i, c := range found {
		cameras[i] = c
	}
	return cameras, nil
}
