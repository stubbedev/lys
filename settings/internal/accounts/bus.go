package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// The accounts service on the system bus: its name, object, interface,
// and the one method that takes an encoded Request.
const (
	BusName      = "dev.stubbe.lys.Users1"
	BusPath      = dbus.ObjectPath("/dev/stubbe/lys/Users1")
	BusInterface = BusName
	ChangeMethod = "Change"

	errorPrefix = BusName + ".Error."
)

// BusErrorName is the D-Bus error an Error of kind travels as: its words
// capitalised and joined under BusName's Error namespace.
func BusErrorName(kind ErrorKind) string {
	var b strings.Builder
	b.WriteString(errorPrefix)
	for word := range strings.SplitSeq(string(kind), "-") {
		if word == "" {
			continue
		}
		b.WriteString(strings.ToUpper(word[:1]))
		b.WriteString(word[1:])
	}
	return b.String()
}

// BusError is e as the service answers a call with it.
func (e *Error) BusError() *dbus.Error {
	return dbus.NewError(BusErrorName(e.Kind), []any{e.Detail})
}

func fromBusError(e dbus.Error) (*Error, bool) {
	for _, k := range ErrorKinds {
		if e.Name != BusErrorName(k) {
			continue
		}
		if len(e.Body) != 1 {
			return nil, false
		}
		detail, ok := e.Body[0].(string)
		return &Error{Kind: k, Detail: detail}, ok
	}
	return nil, false
}

// Change asks the accounts service on conn, the system bus, to make r,
// waiting as long as ctx allows for the caller to authenticate. The
// service's refusal is an *Error; any other failure says what failed.
func Change(ctx context.Context, conn *dbus.Conn, r Request) error {
	body, err := Encode(r)
	if err != nil {
		return err
	}
	call := conn.Object(BusName, BusPath).CallWithContext(ctx, BusInterface+"."+ChangeMethod, dbus.FlagAllowInteractiveAuthorization, body)
	if call.Err == nil {
		return nil
	}
	if reply, ok := errors.AsType[dbus.Error](call.Err); ok {
		if e, ok := fromBusError(reply); ok {
			return e
		}
	}
	return fmt.Errorf("accounts: ask %s to make the %s request: %w", BusName, r.op(), call.Err)
}
