// Package attend answers the idle policy's question before the dim:
// is a face turned to the screen? It looks through the cameras, the
// infrared one first, only while the display session is in the
// foreground and unlocked, and skips the check when a camera is in use.
// Frames stay in memory and are reduced to a presence; nothing is
// logged but the outcome.
package attend

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"

	"github.com/stubbedev/lys/power/internal/facecam"
	"github.com/stubbedev/lys/power/internal/presence"
)

type Session interface {
	Active(ctx context.Context) (bool, error)
	LockedHint(ctx context.Context) (bool, error)
}

type Display func(ctx context.Context) (Session, error)

type Camera interface {
	Look(ctx context.Context) (presence.Look, error)
}

type Checker struct {
	display Display
	cameras []Camera
	log     *slog.Logger
	mu      sync.Mutex
}

func New(display Display, cameras []Camera, log *slog.Logger) *Checker {
	return &Checker{display: display, cameras: cameras, log: log}
}

func (c *Checker) Check(ctx context.Context) (idlepolicy.Presence, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	watching, err := c.foreground(ctx)
	if err != nil || !watching {
		return idlepolicy.Unseen, err
	}
	for _, camera := range c.cameras {
		look, err := camera.Look(ctx)
		switch {
		case errors.Is(err, facecam.ErrBusy):
			c.log.Info("attention check skipped", "reason", err)
			return idlepolicy.Unseen, nil
		case err != nil:
			return 0, err
		}
		if seen, ok := verdict(look); ok {
			return seen, nil
		}
	}
	c.log.Info("attention check skipped", "reason", "every camera too dark to tell")
	return idlepolicy.Unseen, nil
}

func verdict(look presence.Look) (idlepolicy.Presence, bool) {
	switch look {
	case presence.Face:
		return idlepolicy.Present, true
	case presence.NoFace:
		return idlepolicy.Absent, true
	}
	return 0, false
}

func (c *Checker) foreground(ctx context.Context) (bool, error) {
	session, err := c.display(ctx)
	if err != nil {
		return false, fmt.Errorf("find the display session: %w", err)
	}
	active, err := session.Active(ctx)
	if err != nil {
		return false, fmt.Errorf("read whether the display session is active: %w", err)
	}
	if !active {
		c.log.Info("attention check skipped", "reason", "the session is in the background")
		return false, nil
	}
	locked, err := session.LockedHint(ctx)
	if err != nil {
		return false, fmt.Errorf("read whether the display session is locked: %w", err)
	}
	if locked {
		c.log.Info("attention check skipped", "reason", "the session is locked")
		return false, nil
	}
	return true, nil
}
