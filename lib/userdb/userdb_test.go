package userdb_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/userdb"
	"github.com/stubbedev/lys/lib/userdb/userdbtest"
)

var (
	carol   = userdb.User{Name: "carol", UID: 70001, GID: 70001, RealName: "Carol", Home: "/home/carol", Shell: "/bin/zsh"}
	wheel   = userdb.Group{Name: "wheel", GID: 1}
	video   = userdb.Group{Name: "video", GID: 26}
	testsDB = userdbtest.DB{
		Users:       []userdb.User{carol},
		Groups:      []userdb.Group{wheel, video},
		Memberships: map[string][]string{"carol": {"wheel", "video"}},
	}
)

func dial(t *testing.T, path string) *userdb.Conn {
	t.Helper()
	c, err := userdb.Dial(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestUsersGroupsAndMembershipsResolveOnOneConnection(t *testing.T) {
	c := dial(t, userdbtest.Serve(t, testsDB))
	if u, err := c.UserByName(t.Context(), "carol"); err != nil || u != carol {
		t.Errorf("UserByName = %+v, %v; want %+v", u, err, carol)
	}
	if u, err := c.UserByUID(t.Context(), carol.UID); err != nil || u != carol {
		t.Errorf("UserByUID = %+v, %v; want %+v", u, err, carol)
	}
	if g, err := c.GroupByName(t.Context(), "video"); err != nil || g != video {
		t.Errorf("GroupByName = %+v, %v; want %+v", g, err, video)
	}
	if g, err := c.GroupByGID(t.Context(), video.GID); err != nil || g != video {
		t.Errorf("GroupByGID = %+v, %v; want %+v", g, err, video)
	}
	if got, err := c.Memberships(t.Context(), "carol"); err != nil || !slices.Equal(got, []string{"wheel", "video"}) {
		t.Errorf("Memberships = %q, %v; want wheel and video", got, err)
	}
}

func TestLookupUIDResolvesOnAConnectionOfItsOwn(t *testing.T) {
	if u, err := userdb.LookupUID(t.Context(), userdbtest.Serve(t, testsDB), carol.UID); err != nil || u != carol {
		t.Errorf("LookupUID = %+v, %v; want %+v", u, err, carol)
	}
}

func TestAUserInNoGroupHasNoMemberships(t *testing.T) {
	c := dial(t, userdbtest.Serve(t, testsDB))
	if got, err := c.Memberships(t.Context(), "nobody-here"); err != nil || len(got) != 0 {
		t.Errorf("Memberships = %q, %v; want none", got, err)
	}
}

func TestAnUnknownAccountIsNoRecordFound(t *testing.T) {
	c := dial(t, userdbtest.Serve(t, testsDB))
	_, userErr := c.UserByName(t.Context(), "mallory")
	_, uidErr := c.UserByUID(t.Context(), 4242)
	_, groupErr := c.GroupByName(t.Context(), "nogroup")
	_, gidErr := c.GroupByGID(t.Context(), 4343)
	for _, err := range []error{userErr, uidErr, groupErr, gidErr} {
		if !errors.Is(err, userdb.NoRecordFound) {
			t.Errorf("err = %v, want NoRecordFound", err)
		}
		if callErr, ok := errors.AsType[*userdb.CallError](err); !ok || callErr.Name != userdb.NoRecordFound {
			t.Errorf("err = %#v, want a CallError", err)
		}
	}
	if !strings.Contains(uidErr.Error(), "uid=4242") {
		t.Errorf("err = %v, want the uid named", uidErr)
	}
	if !strings.Contains(gidErr.Error(), "gid=4343") {
		t.Errorf("err = %v, want the gid named", gidErr)
	}
}

func TestGroupByGIDAsksForTheGIDAlone(t *testing.T) {
	calls := make(chan userdbtest.Call, 1)
	path := userdbtest.ServeFunc(t, func(c userdbtest.Call) [][]byte {
		calls <- c
		return [][]byte{[]byte(`{"parameters":{"record":{"groupName":"video","gid":26}}}`)}
	})
	if _, err := dial(t, path).GroupByGID(t.Context(), video.GID); err != nil {
		t.Fatal(err)
	}
	c := <-calls
	if c.Method != userdb.MethodGetGroupRecord || string(c.Parameters["gid"]) != "26" || c.Parameters["groupName"] != nil {
		t.Errorf("call = %s %s, want %s with gid 26 and no name", c.Method, c.Parameters, userdb.MethodGetGroupRecord)
	}
}

func TestEveryCallNamesTheMultiplexersService(t *testing.T) {
	calls := make(chan string, 4)
	path := userdbtest.ServeFunc(t, func(c userdbtest.Call) [][]byte {
		calls <- c.String("service")
		return [][]byte{[]byte(`{"error":"io.systemd.UserDatabase.NoRecordFound"}`)}
	})
	c := dial(t, path)
	_, _ = c.UserByUID(t.Context(), 1)
	_, _ = c.GroupByName(t.Context(), "g")
	_, _ = c.GroupByGID(t.Context(), 1)
	_, _ = c.Memberships(t.Context(), "u")
	for range 4 {
		if got := <-calls; got != userdb.Service {
			t.Errorf("service = %q, want %q", got, userdb.Service)
		}
	}
}

func TestMembershipsAskForMoreAndStopAtTheLastReply(t *testing.T) {
	asked := make(chan bool, 1)
	path := userdbtest.ServeFunc(t, func(c userdbtest.Call) [][]byte {
		asked <- c.More
		return [][]byte{
			[]byte(`{"parameters":{"userName":"u","groupName":"a"},"continues":true}`),
			[]byte(`{"parameters":{"userName":"u","groupName":"b"}}`),
			[]byte(`{"parameters":{"userName":"u","groupName":"stray"}}`),
		}
	})
	got, err := dial(t, path).Memberships(t.Context(), "u")
	if more := <-asked; err != nil || !slices.Equal(got, []string{"a", "b"}) || !more {
		t.Errorf("Memberships = %q, %v (more %v); want a and b, asked for more", got, err, more)
	}
}

func TestARecordWithoutAGIDHasItsUIDs(t *testing.T) {
	path := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte {
		return [][]byte{[]byte(`{"parameters":{"record":{"userName":"homed","uid":60100}}}`)}
	})
	u, err := dial(t, path).UserByName(t.Context(), "homed")
	if err != nil || u != (userdb.User{Name: "homed", UID: 60100, GID: 60100}) {
		t.Errorf("UserByName = %+v, %v; want gid 60100", u, err)
	}
}

func TestAMalformedReplyFails(t *testing.T) {
	tests := []struct {
		name, reply, want string
	}{
		{name: "not JSON", reply: `{"parameters":`, want: "malformed reply"},
		{name: "no record", reply: `{"parameters":{}}`, want: "no record"},
		{name: "no user name", reply: `{"parameters":{"record":{"uid":70001}}}`, want: "names no user"},
		{name: "no uid", reply: `{"parameters":{"record":{"userName":"carol"}}}`, want: "no valid uid"},
		{name: "invalid uid", reply: `{"parameters":{"record":{"userName":"carol","uid":4294967295}}}`, want: "no valid uid"},
		{name: "16-bit invalid uid", reply: `{"parameters":{"record":{"userName":"carol","uid":65535}}}`, want: "no valid uid"},
		{name: "negative uid", reply: `{"parameters":{"record":{"userName":"carol","uid":-1}}}`, want: "malformed reply"},
		{name: "invalid gid", reply: `{"parameters":{"record":{"userName":"carol","uid":70001,"gid":65535}}}`, want: "invalid gid"},
		{name: "another user", reply: `{"parameters":{"record":{"userName":"dave","uid":70001}}}`, want: `"dave"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte { return [][]byte{[]byte(tc.reply)} })
			u, err := dial(t, path).UserByName(t.Context(), "carol")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("UserByName = %+v, %v; want an error with %q", u, err, tc.want)
			}
		})
	}
}

func TestAMalformedGroupOrMembershipFails(t *testing.T) {
	groupPath := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte {
		return [][]byte{[]byte(`{"parameters":{"record":{"groupName":"video"}}}`)}
	})
	if g, err := dial(t, groupPath).GroupByName(t.Context(), "video"); err == nil || !strings.Contains(err.Error(), "no valid gid") {
		t.Errorf("GroupByName = %+v, %v; want no valid gid", g, err)
	}
	otherPath := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte {
		return [][]byte{[]byte(`{"parameters":{"record":{"groupName":"audio","gid":17}}}`)}
	})
	if g, err := dial(t, otherPath).GroupByGID(t.Context(), video.GID); err == nil || !strings.Contains(err.Error(), "gid 17's") {
		t.Errorf("GroupByGID = %+v, %v; want the other gid refused", g, err)
	}
	if g, err := dial(t, otherPath).GroupByName(t.Context(), "video"); err == nil || !strings.Contains(err.Error(), `"audio"`) {
		t.Errorf("GroupByName = %+v, %v; want the other group refused", g, err)
	}
	memberPath := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte {
		return [][]byte{[]byte(`{"parameters":{"userName":"someone-else","groupName":"video"}}`)}
	})
	if got, err := dial(t, memberPath).Memberships(t.Context(), "carol"); err == nil || !strings.Contains(err.Error(), "malformed reply") {
		t.Errorf("Memberships = %q, %v; want a malformed reply", got, err)
	}
}

func TestAReplyOverTheLimitFails(t *testing.T) {
	path := userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte {
		return [][]byte{[]byte(`{"parameters":{"record":{"realName":"` + strings.Repeat("x", userdb.MaxMessage) + `"}}}`)}
	})
	if _, err := dial(t, path).UserByName(t.Context(), "carol"); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("err = %v, want the limit named", err)
	}
}

func TestADatabaseThatDoesNotAnswerStopsWithTheContext(t *testing.T) {
	c := dial(t, userdbtest.ServeFunc(t, func(userdbtest.Call) [][]byte { return nil }))
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.UserByName(ctx, "carol"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the deadline", err)
	}
}

func TestAMissingSocketIsAnErrorNamingIt(t *testing.T) {
	path := userdbtest.Missing(t)
	_, err := userdb.Dial(t.Context(), path)
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("Dial = %v, want a missing socket named %s", err, path)
	}
	if _, err := userdb.LookupUID(t.Context(), path, 1); !strings.Contains(err.Error(), path) {
		t.Fatalf("LookupUID = %v, want the socket named", err)
	}
}
