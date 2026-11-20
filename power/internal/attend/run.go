package attend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"

	"github.com/stubbedev/lys/power/attention"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/facecam"
	"github.com/stubbedev/lys/power/internal/presence"
)

func Run(ctx context.Context, cfg config.Config, log *slog.Logger) (err error) {
	cameras, err := FindCameras(cfg.DevDir, facecam.V4L2{})
	if err != nil {
		return err
	}
	system, err := dbusx.DialSystem(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closing("system bus", system.Close())) }()
	sessionBus, err := dbusx.DialSession(ctx, config.BusTimeout)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closing("session bus", sessionBus.Close())) }()
	manager := logind.NewManager(system)
	display := func(ctx context.Context) (Session, error) {
		session, err := manager.GetSession(ctx, logind.AutoSession)
		if err != nil {
			return nil, err
		}
		return session, nil
	}
	release, err := attention.Serve(sessionBus, New(display, cameras, log).Check)
	if err != nil {
		return err
	}
	defer release()
	log.Info("answering attention checks", "cameras", fmt.Sprint(cameras))
	<-ctx.Done()
	return nil
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

func closing(what string, err error) error {
	if err != nil {
		return fmt.Errorf("close the %s: %w", what, err)
	}
	return nil
}
