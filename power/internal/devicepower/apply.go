package devicepower

import (
	"errors"
	"fmt"
)

type Radio interface {
	Stations() ([]Station, error)
	SetPowerSave(s Station, on bool) error
}

func Survey(sys Sysfs, radio Radio) (Inventory, error) {
	inv, err := sys.Read()
	if err != nil {
		return Inventory{}, err
	}
	if radio == nil {
		return inv, nil
	}
	if inv.Stations, err = radio.Stations(); err != nil {
		return Inventory{}, fmt.Errorf("listing Wi-Fi stations: %w", err)
	}
	return inv, nil
}

type Applied struct {
	Decisions []Decision
	Written   []Decision
	Refused   []error
}

func Enforce(sys Sysfs, radio Radio, source Source, rivals []Rival, ov Overrides) (Applied, error) {
	inv, err := Survey(sys, radio)
	if err != nil {
		return Applied{}, err
	}
	return Apply(Plan(inv, source, rivals, ov), radio)
}

func Apply(decisions []Decision, radio Radio) (Applied, error) {
	applied := Applied{Decisions: decisions}
	var errs []error
	for _, d := range decisions {
		if !d.Pending() {
			continue
		}
		err := write(d, radio)
		switch {
		case errors.Is(err, ErrRefused):
			applied.Refused = append(applied.Refused, err)
		case err != nil:
			errs = append(errs, err)
		default:
			applied.Written = append(applied.Written, d)
		}
	}
	return applied, errors.Join(errs...)
}

func write(d Decision, radio Radio) error {
	switch t := d.Target.(type) {
	case File:
		return writeAttr(string(t), d.Want)
	case Station:
		if err := radio.SetPowerSave(t, d.Want == PowerSaveOn); err != nil {
			return fmt.Errorf("setting Wi-Fi power save %s on %s: %w", d.Want, t.Name, err)
		}
		return nil
	default:
		return fmt.Errorf("%s of %s has a target %T no writer knows", d.Kind, d.Subject, t)
	}
}
