// Package proxyenv is a manual proxy as the environment of the programs a
// session starts: the http_proxy, https_proxy, all_proxy and no_proxy
// variables in both cases. A Manual is made only from parsed servers and
// exclusions, so its variables are always ones those programs read.
package proxyenv

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Kind is the traffic a server carries.
type Kind int

// The kinds of server a manual proxy names.
const (
	HTTP Kind = iota
	HTTPS
	SOCKS
	kindCount
)

// Kinds are every kind, in the order a form shows them.
var Kinds = [kindCount]Kind{HTTP, HTTPS, SOCKS}

type kindEnv struct {
	variable string
	scheme   string
}

var kindEnvs = [kindCount]kindEnv{
	HTTP:  {"http_proxy", "http"},
	HTTPS: {"https_proxy", "http"},
	SOCKS: {"all_proxy", "socks5"},
}

const noProxyVariable = "no_proxy"

// The defined refusals.
var (
	ErrHost      = errors.New("a proxy server is a host name or an IP address")
	ErrPort      = errors.New("a proxy port is between 1 and 65535")
	ErrNoServer  = errors.New("a manual proxy names at least one server")
	ErrExclusion = errors.New("an exclusion is a host name, a domain, an IP address, an address range or *")
	ErrMismatch  = errors.New("the lower- and upper-case forms of a proxy variable differ")
	ErrScheme    = errors.New("a proxy variable holds a scheme this form does not write")
	ErrURL       = errors.New("a proxy variable holds more than a scheme, a host and a port")
)

var variableNames = variables()

func variables() []string {
	names := make([]string, 0, 2*(kindCount+1))
	for _, e := range kindEnvs {
		names = append(names, e.variable, strings.ToUpper(e.variable))
	}
	return append(names, noProxyVariable, strings.ToUpper(noProxyVariable))
}

// Server is a proxy server's host and port.
type Server struct {
	host string
	port uint16
}

// ParseServer is host and port as a Server. An IPv6 host may be written in
// brackets.
func ParseServer(host string, port uint16) (Server, error) {
	if port == 0 {
		return Server{}, fmt.Errorf("the port of %q: %w", host, ErrPort)
	}
	if inner, ok := strings.CutPrefix(host, "["); ok {
		host, ok = strings.CutSuffix(inner, "]")
		if !ok {
			return Server{}, fmt.Errorf("the server %q: %w", "["+inner, ErrHost)
		}
	}
	h, ok := parseHost(host)
	if !ok {
		return Server{}, fmt.Errorf("the server %q: %w", host, ErrHost)
	}
	return Server{host: h, port: port}, nil
}

// Host is the server's host name or address, without brackets.
func (s Server) Host() string { return s.host }

// Port is the server's port.
func (s Server) Port() uint16 { return s.port }

func (s Server) url(scheme string) string {
	return scheme + "://" + net.JoinHostPort(s.host, strconv.Itoa(int(s.port)))
}

func parseHost(s string) (string, bool) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.String(), addr.Zone() == ""
	}
	return s, hostName(s)
}

func hostName(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}
	for label := range strings.SplitSeq(s, ".") {
		if !hostLabel(label) {
			return false
		}
	}
	return true
}

func hostLabel(s string) bool {
	if s == "" || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, r := range s {
		switch {
		case r == '-', r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		default:
			return false
		}
	}
	return true
}

// Exclusion is a destination reached without the proxy.
type Exclusion struct{ text string }

// ParseExclusion is s as an Exclusion: *, a host name, a domain written
// with a leading dot or *., an IP address or an address range.
func ParseExclusion(s string) (Exclusion, error) {
	if s != "*" && !validExclusion(s) {
		return Exclusion{}, fmt.Errorf("the exclusion %q: %w", s, ErrExclusion)
	}
	return Exclusion{text: s}, nil
}

func validExclusion(s string) bool {
	if _, err := netip.ParsePrefix(s); err == nil {
		return true
	}
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr.Zone() == ""
	}
	if domain, ok := strings.CutPrefix(s, "*."); ok {
		return hostName(domain)
	}
	return hostName(strings.TrimPrefix(s, "."))
}

func (e Exclusion) String() string { return e.text }

// Manual is a proxy set by hand: a server for some kinds of traffic, and
// the destinations reached without one.
type Manual struct {
	servers [kindCount]Server
	exclude []Exclusion
}

// NewManual is a manual proxy of servers and exclude.
func NewManual(servers map[Kind]Server, exclude []Exclusion) (Manual, error) {
	if len(servers) == 0 {
		return Manual{}, ErrNoServer
	}
	m := Manual{exclude: slices.Clone(exclude)}
	for kind, s := range servers {
		if kind < 0 || kind >= kindCount {
			return Manual{}, fmt.Errorf("proxyenv: a server of kind %d", kind)
		}
		m.servers[kind] = s
	}
	return m, nil
}

// Server is the server of kind, when there is one.
func (m Manual) Server(kind Kind) (Server, bool) {
	s := m.servers[kind]
	return s, s != Server{}
}

// Exclusions are the destinations reached without the proxy.
func (m Manual) Exclusions() []Exclusion { return slices.Clone(m.exclude) }

// Equal reports whether m and o name the same servers and exclusions.
func (m Manual) Equal(o Manual) bool {
	return m.servers == o.servers && slices.Equal(m.exclude, o.exclude)
}

// Env is the variables m sets.
func (m Manual) Env() map[string]string {
	env := make(map[string]string, len(variableNames))
	for kind, e := range kindEnvs {
		if s, ok := m.Server(Kind(kind)); ok {
			setBoth(env, e.variable, s.url(e.scheme))
		}
	}
	if len(m.exclude) > 0 {
		texts := make([]string, len(m.exclude))
		for i, x := range m.exclude {
			texts[i] = x.text
		}
		setBoth(env, noProxyVariable, strings.Join(texts, ","))
	}
	return env
}

func setBoth(env map[string]string, name, value string) {
	env[name] = value
	env[strings.ToUpper(name)] = value
}

// FromEnv is the manual proxy env sets, and whether it sets any of its
// variables.
func FromEnv(env map[string]string) (Manual, bool, error) {
	if !slices.ContainsFunc(variableNames, func(name string) bool { _, ok := env[name]; return ok }) {
		return Manual{}, false, nil
	}
	servers := make(map[Kind]Server, kindCount)
	for kind, e := range kindEnvs {
		v, ok, err := both(env, e.variable)
		if err != nil {
			return Manual{}, true, err
		}
		if !ok {
			continue
		}
		s, err := serverOf(v, e.scheme)
		if err != nil {
			return Manual{}, true, fmt.Errorf("%s=%q: %w", e.variable, v, err)
		}
		servers[Kind(kind)] = s
	}
	v, _, err := both(env, noProxyVariable)
	if err != nil {
		return Manual{}, true, err
	}
	exclude, err := exclusionsOf(v)
	if err != nil {
		return Manual{}, true, fmt.Errorf("%s=%q: %w", noProxyVariable, v, err)
	}
	m, err := NewManual(servers, exclude)
	return m, true, err
}

func both(env map[string]string, name string) (string, bool, error) {
	lower, hasLower := env[name]
	upper, hasUpper := env[strings.ToUpper(name)]
	if hasLower && hasUpper && lower != upper {
		return "", false, fmt.Errorf("%s=%q and %s=%q: %w", name, lower, strings.ToUpper(name), upper, ErrMismatch)
	}
	if hasLower {
		return lower, true, nil
	}
	return upper, hasUpper, nil
}

func serverOf(v, scheme string) (Server, error) {
	u, err := url.Parse(v)
	if err != nil {
		return Server{}, fmt.Errorf("%w: %w", ErrURL, err)
	}
	if u.Scheme != scheme {
		return Server{}, ErrScheme
	}
	if u.User != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return Server{}, ErrURL
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return Server{}, fmt.Errorf("%w: %w", ErrPort, err)
	}
	return ParseServer(u.Hostname(), uint16(port))
}

func exclusionsOf(v string) ([]Exclusion, error) {
	var out []Exclusion
	for part := range strings.SplitSeq(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		x, err := ParseExclusion(part)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, nil
}

// WithoutVariables is env without any variable a manual proxy sets.
func WithoutVariables[V any](env map[string]V) map[string]V {
	out := maps.Clone(env)
	for _, name := range variableNames {
		delete(out, name)
	}
	return out
}
