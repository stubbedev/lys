package devicepower

import (
	"encoding/binary"
	"errors"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func reply(kind uint16, seq uint32, payload []byte) []byte {
	b := binary.NativeEndian.AppendUint32(nil, uint32(unix.NLMSG_HDRLEN+len(payload)))
	b = binary.NativeEndian.AppendUint16(b, kind)
	b = binary.NativeEndian.AppendUint16(b, 0)
	b = binary.NativeEndian.AppendUint32(b, seq)
	b = binary.NativeEndian.AppendUint32(b, 0)
	b = append(b, payload...)
	return append(b, make([]byte, align(len(b))-len(b))...)
}

func errorReply(seq uint32, errno syscall.Errno) []byte {
	code := -int32(errno)
	return reply(unix.NLMSG_ERROR, seq, binary.NativeEndian.AppendUint32(nil, uint32(code)))
}

func attrsOf(t *testing.T, m message) map[uint16][]byte {
	t.Helper()
	attrs, err := parseAttrs(m.payload[unix.GENL_HDRLEN:])
	if err != nil {
		t.Fatal(err)
	}
	return attrs
}

func TestARequestParsesBackToItsAttributes(t *testing.T) {
	req := encodeRequest(0x1c, unix.NLM_F_REQUEST|unix.NLM_F_ACK, 7, unix.NL80211_CMD_SET_POWER_SAVE, []attr{
		uint32Attr(unix.NL80211_ATTR_IFINDEX, 3),
		uint32Attr(unix.NL80211_ATTR_PS_STATE, unix.NL80211_PS_ENABLED),
		stringAttr(unix.NL80211_ATTR_IFNAME, "wlan0"),
	})
	if len(req)%unix.NLA_ALIGNTO != 0 {
		t.Fatalf("request of %d bytes is not aligned", len(req))
	}
	messages, err := parseMessages(req)
	if err != nil || len(messages) != 1 {
		t.Fatalf("parsing the request: %v, %d messages", err, len(messages))
	}
	m := messages[0]
	if m.kind != 0x1c || m.seq != 7 || m.payload[0] != unix.NL80211_CMD_SET_POWER_SAVE {
		t.Fatalf("header %+v", m)
	}
	attrs := attrsOf(t, m)
	if v, err := attrUint32(attrs, unix.NL80211_ATTR_IFINDEX); err != nil || v != 3 {
		t.Errorf("ifindex %d, %v", v, err)
	}
	if v, err := attrUint32(attrs, unix.NL80211_ATTR_PS_STATE); err != nil || v != unix.NL80211_PS_ENABLED {
		t.Errorf("power save %d, %v", v, err)
	}
	if name := string(attrs[unix.NL80211_ATTR_IFNAME]); name != "wlan0\x00" {
		t.Errorf("interface name %q", name)
	}
}

func TestADumpHandsEachReplyOnUntilDone(t *testing.T) {
	station := encodeRequest(0x1c, 0, 1, unix.NL80211_CMD_NEW_INTERFACE, []attr{uint32Attr(unix.NL80211_ATTR_IFTYPE, unix.NL80211_IFTYPE_STATION)})
	messages, err := parseMessages(append(station, reply(unix.NLMSG_DONE, 1, make([]byte, 4))...))
	if err != nil || len(messages) != 2 {
		t.Fatalf("parsing a dump: %v, %d messages", err, len(messages))
	}
	var kind uint32
	done, err := messages[0].handle(func(attrs map[uint16][]byte) error {
		var err error
		kind, err = attrUint32(attrs, unix.NL80211_ATTR_IFTYPE)
		return err
	})
	if done || err != nil || kind != unix.NL80211_IFTYPE_STATION {
		t.Errorf("first message: done %v err %v type %d", done, err, kind)
	}
	if done, err := messages[1].handle(nil); !done || err != nil {
		t.Errorf("NLMSG_DONE: done %v err %v", done, err)
	}
}

func TestAnErrorReplyCarriesTheErrno(t *testing.T) {
	messages, err := parseMessages(errorReply(4, syscall.ENOENT))
	if err != nil {
		t.Fatal(err)
	}
	if done, err := messages[0].handle(nil); !done || !errors.Is(err, syscall.ENOENT) {
		t.Errorf("error reply: done %v err %v, want ENOENT", done, err)
	}
	messages, err = parseMessages(reply(unix.NLMSG_ERROR, 4, make([]byte, 4)))
	if err != nil {
		t.Fatal(err)
	}
	if done, err := messages[0].handle(nil); !done || err != nil {
		t.Errorf("ack: done %v err %v", done, err)
	}
}

func TestATruncatedMessageIsAnError(t *testing.T) {
	full := errorReply(1, syscall.EINVAL)
	for _, cut := range []int{3, unix.NLMSG_HDRLEN - 1, unix.NLMSG_HDRLEN + 2} {
		if _, err := parseMessages(full[:cut]); err == nil {
			t.Errorf("a message cut at %d bytes parsed", cut)
		}
	}
	if _, err := parseAttrs([]byte{8, 0, 1, 0, 0}); err == nil {
		t.Error("an attribute longer than its buffer parsed")
	}
	if _, err := attrUint32(map[uint16][]byte{1: {1, 2}}, 1); err == nil {
		t.Error("a 16-bit attribute read as 32 bits")
	}
}
