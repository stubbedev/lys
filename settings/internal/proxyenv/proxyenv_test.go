package proxyenv

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

func server(t *testing.T, host string, port uint16) Server {
	t.Helper()
	s, err := ParseServer(host, port)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func exclusion(t *testing.T, s string) Exclusion {
	t.Helper()
	x, err := ParseExclusion(s)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestAServerIsAHostNameOrAnAddressAndAPort(t *testing.T) {
	for in, want := range map[string]string{
		"proxy":                 "proxy",
		"proxy.corp.example":    "proxy.corp.example",
		"10.0.0.1":              "10.0.0.1",
		"2001:db8::1":           "2001:db8::1",
		"[2001:db8::1]":         "2001:db8::1",
		"a-1.B-2.example":       "a-1.B-2.example",
		"xn--bcher-kva.example": "xn--bcher-kva.example",
	} {
		s, err := ParseServer(in, 3128)
		if err != nil || s.Host() != want || s.Port() != 3128 {
			t.Errorf("%q: %v %q:%d, want %q:3128", in, err, s.Host(), s.Port(), want)
		}
	}
}

func TestAServerThatCannotBeOneIsRefused(t *testing.T) {
	for in, want := range map[string]error{
		"":                      ErrHost,
		"http://proxy":          ErrHost,
		"proxy:3128":            ErrHost,
		"user@proxy":            ErrHost,
		"pro xy":                ErrHost,
		"-proxy":                ErrHost,
		"proxy-.example":        ErrHost,
		"proxy..example":        ErrHost,
		"proxy.example.":        ErrHost,
		"[2001:db8::1":          ErrHost,
		"fe80::1%eth0":          ErrHost,
		"bücher.example":        ErrHost,
		strings.Repeat("a", 64): ErrHost,
	} {
		if _, err := ParseServer(in, 8080); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", in, err, want)
		}
	}
	if _, err := ParseServer("proxy", 0); !errors.Is(err, ErrPort) {
		t.Errorf("port 0: %v, want %v", err, ErrPort)
	}
}

func TestAnExclusionIsADestinationOrARange(t *testing.T) {
	for _, in := range []string{"*", "localhost", "corp.example", ".corp.example", "*.corp.example", "127.0.0.1", "::1", "10.0.0.0/8", "fd00::/8"} {
		if x, err := ParseExclusion(in); err != nil || x.String() != in {
			t.Errorf("%q: %v %q", in, err, x)
		}
	}
	for _, in := range []string{"", "**", "*corp.example", "http://corp.example", "corp.example:443", "a b", "a,b", "10.0.0.0/33", "..corp"} {
		if _, err := ParseExclusion(in); !errors.Is(err, ErrExclusion) {
			t.Errorf("%q: %v, want %v", in, err, ErrExclusion)
		}
	}
}

func TestAManualProxyNamesAServer(t *testing.T) {
	if _, err := NewManual(nil, []Exclusion{exclusion(t, "localhost")}); !errors.Is(err, ErrNoServer) {
		t.Fatalf("no server: %v, want %v", err, ErrNoServer)
	}
}

func TestAManualProxyIsEveryVariableInBothCases(t *testing.T) {
	m, err := NewManual(map[Kind]Server{
		HTTP:  server(t, "proxy.corp.example", 3128),
		HTTPS: server(t, "2001:db8::1", 3129),
		SOCKS: server(t, "10.0.0.1", 1080),
	}, []Exclusion{exclusion(t, "localhost"), exclusion(t, ".corp.example")})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"http_proxy": "http://proxy.corp.example:3128", "HTTP_PROXY": "http://proxy.corp.example:3128",
		"https_proxy": "http://[2001:db8::1]:3129", "HTTPS_PROXY": "http://[2001:db8::1]:3129",
		"all_proxy": "socks5://10.0.0.1:1080", "ALL_PROXY": "socks5://10.0.0.1:1080",
		"no_proxy": "localhost,.corp.example", "NO_PROXY": "localhost,.corp.example",
	}
	if got := m.Env(); !maps.Equal(got, want) {
		t.Fatalf("env %v, want %v", got, want)
	}
	back, set, err := FromEnv(want)
	if err != nil || !set || !back.Equal(m) {
		t.Fatalf("read back %v %v %+v, want %+v", err, set, back, m)
	}
}

func TestAManualProxySetsOnlyTheVariablesOfWhatItNames(t *testing.T) {
	m, err := NewManual(map[Kind]Server{HTTPS: server(t, "proxy", 8080)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := m.Env(), map[string]string{"https_proxy": "http://proxy:8080", "HTTPS_PROXY": "http://proxy:8080"}; !maps.Equal(got, want) {
		t.Fatalf("env %v, want %v", got, want)
	}
	if _, ok := m.Server(HTTP); ok {
		t.Error("an HTTP server is named")
	}
}

func TestAnEnvironmentWithoutProxyVariablesIsNoManualProxy(t *testing.T) {
	if _, set, err := FromEnv(map[string]string{"EDITOR": "nvim"}); set || err != nil {
		t.Fatalf("set %v, %v", set, err)
	}
}

func TestOneCaseOfAVariableIsRead(t *testing.T) {
	m, set, err := FromEnv(map[string]string{"HTTP_PROXY": "http://proxy:3128/", "no_proxy": " localhost , ,10.0.0.0/8"})
	if err != nil || !set {
		t.Fatalf("set %v, %v", set, err)
	}
	if s, ok := m.Server(HTTP); !ok || s != server(t, "proxy", 3128) {
		t.Errorf("HTTP server %v %v", s, ok)
	}
	if got := m.Exclusions(); len(got) != 2 || got[0].String() != "localhost" || got[1].String() != "10.0.0.0/8" {
		t.Errorf("exclusions %v", got)
	}
}

func TestAnEnvironmentThisFormDoesNotWriteIsRefused(t *testing.T) {
	for _, c := range []struct {
		env  map[string]string
		want error
	}{
		{map[string]string{"http_proxy": "http://a:1", "HTTP_PROXY": "http://b:1"}, ErrMismatch},
		{map[string]string{"https_proxy": "https://proxy:443"}, ErrScheme},
		{map[string]string{"all_proxy": "socks4://proxy:1080"}, ErrScheme},
		{map[string]string{"http_proxy": "proxy:3128"}, ErrScheme},
		{map[string]string{"http_proxy": "http://user:secret@proxy:3128"}, ErrURL},
		{map[string]string{"http_proxy": "http://proxy:3128/path"}, ErrURL},
		{map[string]string{"http_proxy": "http://proxy"}, ErrPort},
		{map[string]string{"http_proxy": "http://proxy:0"}, ErrPort},
		{map[string]string{"http_proxy": "http://proxy:65536"}, ErrPort},
		{map[string]string{"http_proxy": "http://pro_xy:80"}, ErrHost},
		{map[string]string{"http_proxy": "http://proxy:80", "no_proxy": "corp.example:443"}, ErrExclusion},
		{map[string]string{"no_proxy": "localhost"}, ErrNoServer},
	} {
		if _, set, err := FromEnv(c.env); !set || !errors.Is(err, c.want) {
			t.Errorf("%v: set %v, %v, want %v", c.env, set, err, c.want)
		}
	}
}

func TestWithoutVariablesKeepsEverythingElse(t *testing.T) {
	m, err := NewManual(map[Kind]Server{SOCKS: server(t, "proxy", 1080)}, []Exclusion{exclusion(t, "*")})
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]any{"EDITOR": "nvim"}
	for k, v := range m.Env() {
		env[k] = v
	}
	if got := WithoutVariables(env); !maps.Equal(got, map[string]any{"EDITOR": "nvim"}) {
		t.Fatalf("left %v", got)
	}
	if len(env) != 5 {
		t.Fatalf("the input was changed: %v", env)
	}
}
