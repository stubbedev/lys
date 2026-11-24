package accounts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Class is what a request changes, and so which polkit action pkexec
// asks for: the caller's own account, or the others.
type Class string

// The classes of request, as lys-users-helper's first argument.
const (
	Self   Class = "self"
	Others Class = "others"
)

// Classes is every Class.
var Classes = []Class{Self, Others}

// ParseClass is s as a Class.
func ParseClass(s string) (Class, error) {
	for _, c := range Classes {
		if string(c) == s {
			return c, nil
		}
	}
	return "", fmt.Errorf("accounts: no request class %q", s)
}

// Action is the polkit action that authorises requests of c.
func (c Class) Action() string { return "dev.stubbe.lys.settings.users." + string(c) }

// Request is one change lys-users-helper makes.
type Request interface {
	Class() Class
	op() op
	check() error
}

type op string

const (
	opRename  op = "rename"
	opPicture op = "picture"
	opCreate  op = "create"
	opEdit    op = "edit"
	opDelete  op = "delete"
)

// Rename sets the caller's full name.
type Rename struct {
	RealName RealName `json:"realName"`
}

// SetPicture sets the caller's picture.
type SetPicture struct {
	Picture Picture `json:"picture"`
}

// Create adds an account with a password.
type Create struct {
	Login    Login    `json:"login"`
	RealName RealName `json:"realName"`
	Kind     Kind     `json:"kind"`
	Hash     Hash     `json:"hash"`
}

// Edit changes another account: each field that is set.
type Edit struct {
	Login    Name      `json:"login"`
	RealName *RealName `json:"realName,omitempty"`
	Kind     *Kind     `json:"kind,omitempty"`
	Hash     *Hash     `json:"hash,omitempty"`
}

// Delete removes another account, and its home folder with RemoveHome.
type Delete struct {
	Login      Name `json:"login"`
	RemoveHome bool `json:"removeHome"`
}

// Class is Self.
func (Rename) Class() Class { return Self }

// Class is Self.
func (SetPicture) Class() Class { return Self }

// Class is Others.
func (Create) Class() Class { return Others }

// Class is Others.
func (Edit) Class() Class { return Others }

// Class is Others.
func (Delete) Class() Class { return Others }

func (Rename) op() op     { return opRename }
func (SetPicture) op() op { return opPicture }
func (Create) op() op     { return opCreate }
func (Edit) op() op       { return opEdit }
func (Delete) op() op     { return opDelete }

var errNoLogin = errors.New("accounts: the request names no login")

func (Rename) check() error { return nil }

func (r SetPicture) check() error {
	if len(r.Picture) == 0 {
		return errors.New("accounts: the request holds no picture")
	}
	return nil
}

func (r Create) check() error {
	if r.Login == "" {
		return errNoLogin
	}
	if r.Hash == "" {
		return errors.New("accounts: the request holds no password hash")
	}
	return nil
}

func (r Edit) check() error {
	if r.Login == "" {
		return errNoLogin
	}
	return nil
}

func (r Delete) check() error {
	if r.Login == "" {
		return errNoLogin
	}
	return nil
}

type envelope struct {
	Op      op              `json:"op"`
	Request json.RawMessage `json:"request"`
}

// Encode is r as lys-users-helper reads it from its standard input.
func Encode(r Request) ([]byte, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("accounts: encode the %s request: %w", r.op(), err)
	}
	return json.Marshal(envelope{Op: r.op(), Request: body})
}

// Decode reads a request of class from data, refusing one of another
// class, an unknown field, and any value its type refuses.
func Decode(class Class, data []byte) (Request, error) {
	var env envelope
	if err := strictJSON(data, &env); err != nil {
		return nil, fmt.Errorf("accounts: the request: %w", err)
	}
	var r Request
	var err error
	switch env.Op {
	case opRename:
		r, err = decodeAs[Rename](env.Request)
	case opPicture:
		r, err = decodeAs[SetPicture](env.Request)
	case opCreate:
		r, err = decodeAs[Create](env.Request)
	case opEdit:
		r, err = decodeAs[Edit](env.Request)
	case opDelete:
		r, err = decodeAs[Delete](env.Request)
	default:
		return nil, fmt.Errorf("accounts: no request %q", env.Op)
	}
	if err != nil {
		return nil, fmt.Errorf("accounts: the %s request: %w", env.Op, err)
	}
	if r.Class() != class {
		return nil, fmt.Errorf("accounts: a %s request is one of %s, not %s", env.Op, r.Class(), class)
	}
	return r, r.check()
}

func decodeAs[T Request](data []byte) (Request, error) {
	var v T
	if err := strictJSON(data, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func strictJSON(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.More() {
		return errors.New("more than one value")
	}
	return nil
}

// ErrorKind is why lys-users-helper refused or failed a request.
type ErrorKind string

// The kinds of Error.
const (
	// ErrInvalid is a request that does not decode or is not one the
	// helper's caller may make.
	ErrInvalid ErrorKind = "invalid"
	// ErrUnknownUser is a request about an account that does not exist.
	ErrUnknownUser ErrorKind = "unknown-user"
	// ErrNotAPerson is a request about a system account.
	ErrNotAPerson ErrorKind = "not-a-person"
	// ErrOwnAccount is a request to delete the caller's own account.
	ErrOwnAccount ErrorKind = "own-account"
	// ErrTaken is a new account's login name already being one.
	ErrTaken ErrorKind = "taken"
	// ErrNoAdmins is an administrator asked for where no group makes one.
	ErrNoAdmins ErrorKind = "no-admin-group"
	// ErrFailed is a tool that failed to make the change.
	ErrFailed ErrorKind = "failed"
	// ErrNotAuthorized is pkexec refusing the caller, or the caller
	// dismissing the authentication; the helper never ran.
	ErrNotAuthorized ErrorKind = "not-authorized"
)

// Error is lys-users-helper's answer to a request it did not carry out,
// written as JSON to its standard output.
type Error struct {
	Kind   ErrorKind `json:"kind"`
	Detail string    `json:"detail"`
}

func (e *Error) Error() string { return string(e.Kind) + ": " + e.Detail }

// Errorf is an Error of kind saying what format says.
func Errorf(kind ErrorKind, format string, args ...any) *Error {
	return &Error{Kind: kind, Detail: fmt.Sprintf(format, args...)}
}

// HelperName is the program that makes the changes, in the libexec
// directory beside lys-settings' bin directory.
const HelperName = "lys-users-helper"

// The exit statuses of pkexec that mean the helper never ran: the
// authentication dialog dismissed, and the caller not authorised.
const (
	pkexecDismissed     = 126
	pkexecNotAuthorized = 127
)

// Outcome is what a run of `pkexec lys-users-helper` that exited with
// code says: nil for 0, the helper's Error for 1, ErrNotAuthorized for
// pkexec's refusals, and an error naming the code and stderr otherwise.
func Outcome(code int, stdout, stderr []byte) error {
	said := string(bytes.TrimSpace(stderr))
	switch code {
	case 0:
		return nil
	case 1:
		var e Error
		if err := json.Unmarshal(stdout, &e); err != nil || e.Kind == "" {
			return fmt.Errorf("%s failed without saying why: %s", HelperName, said)
		}
		return &e
	case pkexecDismissed:
		return Errorf(ErrNotAuthorized, "the authentication was dismissed")
	case pkexecNotAuthorized:
		return Errorf(ErrNotAuthorized, "pkexec refused: %s", said)
	}
	return fmt.Errorf("%s exited %d: %s", HelperName, code, said)
}
