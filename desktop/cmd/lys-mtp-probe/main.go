// Command lys-mtp-probe tells udev whether a USB device is a camera or a
// phone lys-files opens over desktop/mtp. The rule beside it,
// 69-lys-mtp.rules, runs it as IMPORT{program} with the device's kernel
// name and tags the device uaccess on what it prints: ID_LYS_USB_PROTOCOL
// PTP for a still image interface, MTP for an MTP one, with ID_MTP_DEVICE=1
// as libmtp's rules set it; nothing for any other device. A vendor-specific
// interface shaped like Android's MTP interface is asked for its Microsoft
// OS descriptor, as desktop/mtp asks it.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/mtp/usb"
)

const (
	protocolKey  = "ID_LYS_USB_PROTOCOL"
	mtpDeviceKey = "ID_MTP_DEVICE"
	probeTimeout = 10 * time.Second
)

const (
	exitFailed = 1
	exitUsage  = 2
)

type prober func(context.Context, usb.Device) (bool, error)

func main() { os.Exit(run(os.Args[1:], usb.SysfsDevices, usb.Probe, os.Stdout, os.Stderr)) }

// run prints the udev properties of the device args name under root and
// is the status to exit with, having said on stderr what went wrong.
func run(args []string, root string, probe prober, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: lys-mtp-probe <kernel name of a USB device>")
		return exitUsage
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	props, err := properties(ctx, root, args[0], probe)
	if err == nil && len(props) > 0 {
		_, err = io.WriteString(stdout, strings.Join(props, "\n")+"\n")
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lys-mtp-probe: %s: %v\n", args[0], err)
		return exitFailed
	}
	return 0
}

func properties(ctx context.Context, root, name string, probe prober) ([]string, error) {
	d, ok, err := usb.Lookup(root, name)
	if err != nil || !ok {
		return nil, err
	}
	if d.Protocol == usb.Vendor {
		isMTP, err := probe(ctx, d)
		if err != nil || !isMTP {
			return nil, err
		}
		d.Protocol = usb.MTP
	}
	props := []string{protocolKey + "=" + d.Protocol.String()}
	if d.Protocol == usb.MTP {
		props = append(props, mtpDeviceKey+"=1")
	}
	return props, nil
}
