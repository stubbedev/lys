// Package userdb resolves users and groups through systemd's
// io.systemd.UserDatabase Varlink interface on the multiplexer socket,
// which answers from every source systemd-userdbd knows: the NSS
// bridge (so every nsswitch.conf source), drop-in records and
// systemd-homed. It speaks Varlink itself, pure Go, and a missing
// socket is an error naming it, never a fallback to /etc/passwd.
package userdb

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"time"
)

// SocketPath is the multiplexer's socket, served by systemd-userdbd.socket.
const SocketPath = "/run/systemd/userdb/io.systemd.Multiplexer"

// Service is the service parameter every call to the multiplexer names.
const Service = "io.systemd.Multiplexer"

// The interface's methods.
const (
	MethodGetUserRecord  = "io.systemd.UserDatabase.GetUserRecord"
	MethodGetGroupRecord = "io.systemd.UserDatabase.GetGroupRecord"
	MethodGetMemberships = "io.systemd.UserDatabase.GetMemberships"
)

// CallTimeout bounds every call, whatever the caller's context allows: a
// user database that does not answer never hangs its caller.
const CallTimeout = 10 * time.Second

// MaxMessage is the largest reply accepted, NUL terminator included.
const MaxMessage = 1 << 20

// ErrorName is a Varlink error the user database replies with. It is an
// error itself, so errors.Is(err, NoRecordFound) tells a failed call apart.
type ErrorName string

// Error is the Varlink error's name.
func (n ErrorName) Error() string { return string(n) }

// The errors io.systemd.UserDatabase defines.
const (
	NoRecordFound           ErrorName = "io.systemd.UserDatabase.NoRecordFound"
	BadService              ErrorName = "io.systemd.UserDatabase.BadService"
	ServiceNotAvailable     ErrorName = "io.systemd.UserDatabase.ServiceNotAvailable"
	ConflictingRecordFound  ErrorName = "io.systemd.UserDatabase.ConflictingRecordFound"
	NonMatchingRecordFound  ErrorName = "io.systemd.UserDatabase.NonMatchingRecordFound"
	EnumerationNotSupported ErrorName = "io.systemd.UserDatabase.EnumerationNotSupported"
)

// CallError is a call the user database answered with an error.
type CallError struct {
	Method string
	// Key is what the call looked up, as "userName=ada" or "uid=1000".
	Key  string
	Name ErrorName
}

func (e *CallError) Error() string {
	return fmt.Sprintf("userdb %s %s: %s", e.Method, e.Key, e.Name)
}

// Unwrap is the error's name.
func (e *CallError) Unwrap() error { return e.Name }

// UID is a user ID.
type UID uint32

// GID is a group ID.
type GID uint32

const (
	invalidID   = math.MaxUint32
	invalidID16 = math.MaxUint16
)

func validID(id uint32) bool { return id != invalidID && id != invalidID16 }

// User is the part of a JSON user record lys reads.
type User struct {
	Name string `json:"userName"`
	UID  UID    `json:"uid"`
	// GID is the primary group; a record that names none has its UID's.
	GID      GID    `json:"gid"`
	RealName string `json:"realName,omitempty"`
	// Home and Shell are "" where the record names none.
	Home  string `json:"homeDirectory,omitempty"`
	Shell string `json:"shell,omitempty"`
}

// UnmarshalJSON parses a user record, which must name the user and a
// valid UID.
func (u *User) UnmarshalJSON(data []byte) error {
	type record User
	r := record{UID: invalidID, GID: invalidID}
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("user record: %w", err)
	}
	if r.Name == "" {
		return errors.New("user record names no user")
	}
	if !validID(uint32(r.UID)) {
		return fmt.Errorf("user record %q has no valid uid", r.Name)
	}
	if r.GID == invalidID {
		r.GID = GID(r.UID)
	}
	if !validID(uint32(r.GID)) {
		return fmt.Errorf("user record %q has an invalid gid %d", r.Name, r.GID)
	}
	*u = User(r)
	return nil
}

// Group is the part of a JSON group record lys reads.
type Group struct {
	Name string `json:"groupName"`
	GID  GID    `json:"gid"`
}

// UnmarshalJSON parses a group record, which must name the group and a
// valid GID.
func (g *Group) UnmarshalJSON(data []byte) error {
	type record Group
	r := record{GID: invalidID}
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("group record: %w", err)
	}
	if r.Name == "" {
		return errors.New("group record names no group")
	}
	if !validID(uint32(r.GID)) {
		return fmt.Errorf("group record %q has no valid gid", r.Name)
	}
	*g = Group(r)
	return nil
}

// Conn is one connection to the user database. Its calls run one at a
// time.
type Conn struct {
	conn net.Conn
	r    *bufio.Reader
	path string
}

// Dial connects to the user database at path, normally SocketPath.
func Dial(ctx context.Context, path string) (*Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("connect to the systemd user database at %s (is systemd-userdbd.socket running?): %w", path, err)
	}
	return &Conn{conn: conn, r: bufio.NewReader(conn), path: path}, nil
}

// Close closes the connection.
func (c *Conn) Close() error { return c.conn.Close() }

// LookupUID is uid's user record, from a connection of its own to the
// user database at path.
func LookupUID(ctx context.Context, path string, uid UID) (User, error) {
	c, err := Dial(ctx, path)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = c.Close() }()
	return c.UserByUID(ctx, uid)
}

// UserByName is the user record of the user called name.
func (c *Conn) UserByName(ctx context.Context, name string) (User, error) {
	u, err := c.user(ctx, "userName="+name, map[string]any{"userName": name})
	if err == nil && u.Name != name {
		return User{}, fmt.Errorf("userdb %s userName=%s: the record is %q's", MethodGetUserRecord, name, u.Name)
	}
	return u, err
}

// UserByUID is the user record of uid.
func (c *Conn) UserByUID(ctx context.Context, uid UID) (User, error) {
	key := "uid=" + strconv.FormatUint(uint64(uid), 10)
	u, err := c.user(ctx, key, map[string]any{"uid": uid})
	if err == nil && u.UID != uid {
		return User{}, fmt.Errorf("userdb %s %s: the record is uid %d's", MethodGetUserRecord, key, u.UID)
	}
	return u, err
}

func (c *Conn) user(ctx context.Context, key string, params map[string]any) (User, error) {
	var reply struct {
		Record *User `json:"record"`
	}
	if err := c.one(ctx, MethodGetUserRecord, key, params, &reply); err != nil {
		return User{}, err
	}
	if reply.Record == nil {
		return User{}, fmt.Errorf("userdb %s %s: the reply holds no record", MethodGetUserRecord, key)
	}
	return *reply.Record, nil
}

// GroupByName is the group record of the group called name.
func (c *Conn) GroupByName(ctx context.Context, name string) (Group, error) {
	g, err := c.group(ctx, "groupName="+name, map[string]any{"groupName": name})
	if err == nil && g.Name != name {
		return Group{}, fmt.Errorf("userdb %s groupName=%s: the record is %q's", MethodGetGroupRecord, name, g.Name)
	}
	return g, err
}

// GroupByGID is the group record of gid.
func (c *Conn) GroupByGID(ctx context.Context, gid GID) (Group, error) {
	key := "gid=" + strconv.FormatUint(uint64(gid), 10)
	g, err := c.group(ctx, key, map[string]any{"gid": gid})
	if err == nil && g.GID != gid {
		return Group{}, fmt.Errorf("userdb %s %s: the record is gid %d's", MethodGetGroupRecord, key, g.GID)
	}
	return g, err
}

func (c *Conn) group(ctx context.Context, key string, params map[string]any) (Group, error) {
	var reply struct {
		Record *Group `json:"record"`
	}
	if err := c.one(ctx, MethodGetGroupRecord, key, params, &reply); err != nil {
		return Group{}, err
	}
	if reply.Record == nil {
		return Group{}, fmt.Errorf("userdb %s %s: the reply holds no record", MethodGetGroupRecord, key)
	}
	return *reply.Record, nil
}

// Memberships is the names of the groups user is a member of besides
// its primary group, in the order the database lists them; none is an
// empty list.
func (c *Conn) Memberships(ctx context.Context, user string) ([]string, error) {
	key := "userName=" + user
	var groups []string
	err := c.call(ctx, MethodGetMemberships, key, map[string]any{"userName": user}, true, func(params json.RawMessage) error {
		var m struct {
			UserName  string `json:"userName"`
			GroupName string `json:"groupName"`
		}
		if err := json.Unmarshal(params, &m); err != nil {
			return err
		}
		if m.UserName != user || m.GroupName == "" {
			return fmt.Errorf("membership %s", params)
		}
		groups = append(groups, m.GroupName)
		return nil
	})
	if errors.Is(err, NoRecordFound) {
		return nil, nil
	}
	return groups, err
}

func (c *Conn) one(ctx context.Context, method, key string, params map[string]any, into any) error {
	return c.call(ctx, method, key, params, false, func(raw json.RawMessage) error { return json.Unmarshal(raw, into) })
}

type request struct {
	Method     string         `json:"method"`
	Parameters map[string]any `json:"parameters"`
	More       bool           `json:"more,omitempty"`
}

type reply struct {
	Parameters json.RawMessage `json:"parameters"`
	Continues  bool            `json:"continues"`
	Error      ErrorName       `json:"error"`
}

func (c *Conn) call(ctx context.Context, method, key string, params map[string]any, more bool, each func(json.RawMessage) error) error {
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = c.conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()
	fail := func(err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = ctxErr
		}
		return fmt.Errorf("userdb %s %s on %s: %w", method, key, c.path, err)
	}
	params["service"] = Service
	msg, err := json.Marshal(request{Method: method, Parameters: params, More: more})
	if err != nil {
		return fail(err)
	}
	if _, err := c.conn.Write(append(msg, 0)); err != nil {
		return fail(err)
	}
	for {
		frame, err := c.readFrame()
		if err != nil {
			return fail(err)
		}
		var r reply
		if err := json.Unmarshal(frame, &r); err != nil {
			return fail(fmt.Errorf("malformed reply %q: %w", frame, err))
		}
		if r.Error != "" {
			return &CallError{Method: method, Key: key, Name: r.Error}
		}
		if err := each(r.Parameters); err != nil {
			return fail(fmt.Errorf("malformed reply %q: %w", frame, err))
		}
		if !more || !r.Continues {
			return nil
		}
	}
}

func (c *Conn) readFrame() ([]byte, error) {
	var frame []byte
	for {
		chunk, err := c.r.ReadSlice(0)
		if len(frame)+len(chunk) > MaxMessage {
			return nil, fmt.Errorf("a reply over the %d-byte limit", MaxMessage)
		}
		frame = append(frame, chunk...)
		switch {
		case err == nil:
			return frame[:len(frame)-1], nil
		case errors.Is(err, bufio.ErrBufferFull):
		case errors.Is(err, io.EOF):
			return nil, io.ErrUnexpectedEOF
		default:
			return nil, err
		}
	}
}
