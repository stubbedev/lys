package main

import (
	"fmt"
	"strings"

	"github.com/stubbedev/lys/power/internal/wakesource"
)

func appendWake(out []byte, mode wakesource.Mode, acpi []wakesource.ACPIDevice, usb []wakesource.USBDevice, err error) []byte {
	out = fmt.Appendf(out, "wake:        USB wake policy %s", mode)
	if mode == wakesource.ModeKeep {
		out = fmt.Appendf(out, " (LYS_POWER_USB_WAKE=%s keeps removable USB devices from waking the machine on battery)", wakesource.ModeBattery)
	}
	out = append(out, '\n')
	if err != nil {
		return fmt.Appendf(out, "             %v\n", err)
	}
	for _, d := range acpi {
		if d.Enabled {
			out = fmt.Appendf(out, "  acpi %-8s %-3s %s\n", d.Name, d.State, strings.Join(d.Nodes, ", "))
		}
	}
	for _, d := range usb {
		if !d.Enabled() {
			continue
		}
		kind := d.Removable.String()
		if d.Hub {
			kind = "hub"
		}
		out = fmt.Appendf(out, "  usb  %-8s %-9s %s\n", d.Name, kind, d.Product)
	}
	return out
}
