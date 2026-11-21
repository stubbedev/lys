// Package sdnotify reports a service's state to the supervisor that
// started it, as sd_notify does for systemd's Type=notify services.
package sdnotify

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Env names the socket systemd's Type=notify services report on.
const Env = "NOTIFY_SOCKET"

const notifyTimeout = 5 * time.Second

// Notify sends state, such as "READY=1", to the notify socket at addr, as
// sd_notify does; "@" leads an abstract address. With addr empty there is no
// supervisor listening and Notify does nothing.
func Notify(addr, state string) error {
	if addr == "" {
		return nil
	}
	c, err := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: addr, Net: "unixgram"})
	if err != nil {
		return fmt.Errorf("sdnotify: reach the notify socket %s: %w", addr, err)
	}
	err = send(c, state)
	if cerr := c.Close(); cerr != nil {
		err = errors.Join(err, fmt.Errorf("sdnotify: close the notify socket %s: %w", addr, cerr))
	}
	if err != nil {
		return fmt.Errorf("sdnotify: notify %s of %q: %w", addr, state, err)
	}
	return nil
}

func send(c *net.UnixConn, state string) error {
	if err := c.SetWriteDeadline(time.Now().Add(notifyTimeout)); err != nil {
		return err
	}
	_, err := c.Write([]byte(state))
	return err
}
