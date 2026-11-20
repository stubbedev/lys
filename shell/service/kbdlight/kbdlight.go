// Package kbdlight is the one owner of the keyboard backlights: they are
// off while the session is idle or the room is bright, and otherwise back
// at the level each had when they went off.
package kbdlight

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Level is one keyboard backlight's raw brightness.
type Level struct {
	Name string
	Raw  uint32
}

// LEDs reads and sets the keyboard backlights.
type LEDs interface {
	Levels(ctx context.Context) ([]Level, error)
	Set(ctx context.Context, name string, raw uint32) error
}

// Reason is why the keyboard backlights are off.
type Reason uint8

// The reasons.
const (
	Inactive Reason = 1 << iota
	Bright
)

// Light turns the keyboard backlights off while any reason holds and puts
// back the levels it found once none does.
type Light struct {
	leds LEDs

	mu      sync.Mutex
	reasons Reason
	kept    []Level
}

// New is a light over leds with every backlight on.
func New(leds LEDs) *Light { return &Light{leds: leds} }

// Hold turns the backlights off for reason, or lets go of it.
func (l *Light) Hold(ctx context.Context, reason Reason, on bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	was := l.reasons
	if on {
		l.reasons |= reason
	} else {
		l.reasons &^= reason
	}
	switch {
	case was == 0 && l.reasons != 0:
		return l.off(ctx)
	case was != 0 && l.reasons == 0:
		return l.restore(ctx)
	}
	return nil
}

func (l *Light) off(ctx context.Context) error {
	levels, err := l.leds.Levels(ctx)
	if err != nil {
		return fmt.Errorf("kbdlight: read the keyboard backlights: %w", err)
	}
	l.kept = levels
	var errs []error
	for _, lv := range levels {
		if lv.Raw == 0 {
			continue
		}
		if err := l.leds.Set(ctx, lv.Name, 0); err != nil {
			errs = append(errs, fmt.Errorf("kbdlight: turn %s off: %w", lv.Name, err))
		}
	}
	return errors.Join(errs...)
}

func (l *Light) restore(ctx context.Context) error {
	kept := l.kept
	l.kept = nil
	var errs []error
	for _, lv := range kept {
		if lv.Raw == 0 {
			continue
		}
		if err := l.leds.Set(ctx, lv.Name, lv.Raw); err != nil {
			errs = append(errs, fmt.Errorf("kbdlight: turn %s back on at %d: %w", lv.Name, lv.Raw, err))
		}
	}
	return errors.Join(errs...)
}
