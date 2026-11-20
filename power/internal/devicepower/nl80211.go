package devicepower

import (
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	"golang.org/x/sys/unix"
)

var ErrNoWiFi = errors.New("the kernel has no nl80211 family")

const (
	netlinkTimeout = 2 * time.Second
	netlinkBuffer  = 1 << 16
	attrTypeMask   = ^uint16(unix.NLA_F_NESTED | unix.NLA_F_NET_BYTEORDER)
)

type NL80211 struct {
	fd     int
	family uint16
	seq    uint32
}

func DialNL80211() (*NL80211, error) {
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC, unix.NETLINK_GENERIC)
	if err != nil {
		return nil, fmt.Errorf("opening a generic netlink socket: %w", err)
	}
	n := &NL80211{fd: fd}
	if err := n.setup(); err != nil {
		return nil, errors.Join(err, n.Close())
	}
	return n, nil
}

func (n *NL80211) setup() error {
	timeout := unix.NsecToTimeval(netlinkTimeout.Nanoseconds())
	for _, opt := range []int{unix.SO_RCVTIMEO, unix.SO_SNDTIMEO} {
		if err := unix.SetsockoptTimeval(n.fd, unix.SOL_SOCKET, opt, &timeout); err != nil {
			return fmt.Errorf("bounding generic netlink socket I/O to %s: %w", netlinkTimeout, err)
		}
	}
	if err := unix.Bind(n.fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("binding the generic netlink socket: %w", err)
	}
	found := false
	err := n.exchange(unix.GENL_ID_CTRL, 0, unix.CTRL_CMD_GETFAMILY, []attr{stringAttr(unix.CTRL_ATTR_FAMILY_NAME, "nl80211")}, func(attrs map[uint16][]byte) error {
		id, err := attrUint16(attrs, unix.CTRL_ATTR_FAMILY_ID)
		if err != nil {
			return err
		}
		n.family, found = id, true
		return nil
	})
	if errors.Is(err, syscall.ENOENT) {
		return ErrNoWiFi
	}
	if err != nil {
		return fmt.Errorf("resolving the nl80211 family: %w", err)
	}
	if !found {
		return errors.New("resolving the nl80211 family: the reply carries no family ID")
	}
	return nil
}

func (n *NL80211) Close() error {
	if err := unix.Close(n.fd); err != nil {
		return fmt.Errorf("closing the generic netlink socket: %w", err)
	}
	return nil
}

func (n *NL80211) Stations() ([]Station, error) {
	var stations []Station
	err := n.exchange(n.family, unix.NLM_F_DUMP, unix.NL80211_CMD_GET_INTERFACE, nil, func(attrs map[uint16][]byte) error {
		kind, err := attrUint32(attrs, unix.NL80211_ATTR_IFTYPE)
		if err != nil || kind != unix.NL80211_IFTYPE_STATION {
			return err
		}
		index, err := attrUint32(attrs, unix.NL80211_ATTR_IFINDEX)
		if err != nil {
			return err
		}
		name, ok := attrs[unix.NL80211_ATTR_IFNAME]
		if !ok {
			return fmt.Errorf("interface %d has no name", index)
		}
		stations = append(stations, Station{Name: strings.TrimRight(string(name), "\x00"), Index: int(index)})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing wireless interfaces: %w", err)
	}
	for i := range stations {
		if stations[i].PowerSave, err = n.powerSave(stations[i]); err != nil {
			return nil, err
		}
	}
	slices.SortFunc(stations, func(a, b Station) int { return strings.Compare(a.Name, b.Name) })
	return stations, nil
}

func (n *NL80211) powerSave(s Station) (Value, error) {
	index, err := intconv.Exact[uint32](s.Index)
	if err != nil {
		return "", fmt.Errorf("interface index of %s: %w", s.Name, err)
	}
	state := PowerSaveOff
	found := false
	err = n.exchange(n.family, 0, unix.NL80211_CMD_GET_POWER_SAVE, []attr{uint32Attr(unix.NL80211_ATTR_IFINDEX, index)}, func(attrs map[uint16][]byte) error {
		ps, err := attrUint32(attrs, unix.NL80211_ATTR_PS_STATE)
		if err != nil {
			return err
		}
		if ps == unix.NL80211_PS_ENABLED {
			state = PowerSaveOn
		}
		found = true
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("reading the power save state of %s: %w", s.Name, err)
	}
	if !found {
		return "", fmt.Errorf("reading the power save state of %s: no reply", s.Name)
	}
	return state, nil
}

func (n *NL80211) SetPowerSave(s Station, on bool) error {
	index, err := intconv.Exact[uint32](s.Index)
	if err != nil {
		return fmt.Errorf("interface index of %s: %w", s.Name, err)
	}
	state := uint32(unix.NL80211_PS_DISABLED)
	if on {
		state = unix.NL80211_PS_ENABLED
	}
	return n.exchange(n.family, 0, unix.NL80211_CMD_SET_POWER_SAVE, []attr{
		uint32Attr(unix.NL80211_ATTR_IFINDEX, index),
		uint32Attr(unix.NL80211_ATTR_PS_STATE, state),
	}, func(map[uint16][]byte) error { return nil })
}

func (n *NL80211) exchange(family, flags uint16, cmd uint8, attrs []attr, each func(map[uint16][]byte) error) error {
	n.seq++
	req := encodeRequest(family, flags|unix.NLM_F_REQUEST|unix.NLM_F_ACK, n.seq, cmd, attrs)
	if err := unix.Sendto(n.fd, req, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return fmt.Errorf("sending generic netlink command %d: %w", cmd, err)
	}
	buf := make([]byte, netlinkBuffer)
	for {
		size, _, err := unix.Recvfrom(n.fd, buf, 0)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("receiving the reply to generic netlink command %d: %w", cmd, err)
		}
		messages, err := parseMessages(buf[:size])
		if err != nil {
			return err
		}
		for _, m := range messages {
			if m.seq != n.seq {
				continue
			}
			done, err := m.handle(each)
			if done || err != nil {
				return err
			}
		}
	}
}

type attr struct {
	kind uint16
	data []byte
}

func uint32Attr(kind uint16, v uint32) attr {
	return attr{kind: kind, data: binary.NativeEndian.AppendUint32(nil, v)}
}

func stringAttr(kind uint16, s string) attr {
	return attr{kind: kind, data: append([]byte(s), 0)}
}

func align(n int) int { return (n + unix.NLA_ALIGNTO - 1) &^ (unix.NLA_ALIGNTO - 1) }

func encodeRequest(family, flags uint16, seq uint32, cmd uint8, attrs []attr) []byte {
	size := unix.NLMSG_HDRLEN + unix.GENL_HDRLEN
	for _, a := range attrs {
		size += align(unix.NLA_HDRLEN + len(a.data))
	}
	b := make([]byte, 0, size)
	b = binary.NativeEndian.AppendUint32(b, intconv.Must[uint32](size))
	b = binary.NativeEndian.AppendUint16(b, family)
	b = binary.NativeEndian.AppendUint16(b, flags)
	b = binary.NativeEndian.AppendUint32(b, seq)
	b = binary.NativeEndian.AppendUint32(b, 0)
	b = append(b, cmd, 1, 0, 0)
	for _, a := range attrs {
		b = binary.NativeEndian.AppendUint16(b, intconv.Must[uint16](unix.NLA_HDRLEN+len(a.data)))
		b = binary.NativeEndian.AppendUint16(b, a.kind)
		b = append(b, a.data...)
		b = append(b, make([]byte, align(len(a.data))-len(a.data))...)
	}
	return b
}

type message struct {
	kind    uint16
	seq     uint32
	payload []byte
}

func parseMessages(b []byte) ([]message, error) {
	var out []message
	for len(b) > 0 {
		if len(b) < unix.NLMSG_HDRLEN {
			return nil, fmt.Errorf("netlink message header cut short at %d bytes", len(b))
		}
		size := int(binary.NativeEndian.Uint32(b))
		if size < unix.NLMSG_HDRLEN || size > len(b) {
			return nil, fmt.Errorf("netlink message claims %d bytes of %d", size, len(b))
		}
		out = append(out, message{
			kind:    binary.NativeEndian.Uint16(b[4:]),
			seq:     binary.NativeEndian.Uint32(b[8:]),
			payload: b[unix.NLMSG_HDRLEN:size],
		})
		b = b[min(align(size), len(b)):]
	}
	return out, nil
}

func (m message) handle(each func(map[uint16][]byte) error) (bool, error) {
	switch m.kind {
	case unix.NLMSG_DONE, unix.NLMSG_ERROR:
		if len(m.payload) < 4 {
			return true, fmt.Errorf("netlink message %d carries no error code", m.kind)
		}
		if code := intconv.Int32Bits(binary.NativeEndian.Uint32(m.payload)); code < 0 {
			return true, syscall.Errno(-code)
		}
		return true, nil
	}
	if len(m.payload) < unix.GENL_HDRLEN {
		return true, fmt.Errorf("generic netlink reply of %d bytes has no header", len(m.payload))
	}
	attrs, err := parseAttrs(m.payload[unix.GENL_HDRLEN:])
	if err != nil {
		return true, err
	}
	return false, each(attrs)
}

func parseAttrs(b []byte) (map[uint16][]byte, error) {
	attrs := map[uint16][]byte{}
	for len(b) > 0 {
		if len(b) < unix.NLA_HDRLEN {
			return nil, fmt.Errorf("netlink attribute header cut short at %d bytes", len(b))
		}
		size := int(binary.NativeEndian.Uint16(b))
		if size < unix.NLA_HDRLEN || size > len(b) {
			return nil, fmt.Errorf("netlink attribute claims %d bytes of %d", size, len(b))
		}
		attrs[binary.NativeEndian.Uint16(b[2:])&attrTypeMask] = b[unix.NLA_HDRLEN:size]
		b = b[min(align(size), len(b)):]
	}
	return attrs, nil
}

func attrUint32(attrs map[uint16][]byte, kind uint16) (uint32, error) {
	data, ok := attrs[kind]
	if !ok || len(data) != 4 {
		return 0, fmt.Errorf("netlink attribute %d is missing or not 32 bits", kind)
	}
	return binary.NativeEndian.Uint32(data), nil
}

func attrUint16(attrs map[uint16][]byte, kind uint16) (uint16, error) {
	data, ok := attrs[kind]
	if !ok || len(data) != 2 {
		return 0, fmt.Errorf("netlink attribute %d is missing or not 16 bits", kind)
	}
	return binary.NativeEndian.Uint16(data), nil
}
