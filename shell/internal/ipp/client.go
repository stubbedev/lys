package ipp

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/lib/atomicfile"
)

// The CUPS scheduler's domain sockets, the default server where one
// exists (libcups' cupsServer order).
var defaultSockets = []string{"/run/cups/cups.sock", "/var/run/cups/cups.sock"}

// DefaultServer is libcups' server choice: $CUPS_SERVER (a socket path
// or host[:port]), else the scheduler's domain socket, else
// localhost:631.
func DefaultServer() string {
	if s := os.Getenv("CUPS_SERVER"); s != "" {
		return s
	}
	for _, s := range defaultSockets {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	return "localhost:631"
}

// Client talks IPP to one CUPS server.
type Client struct {
	http *http.Client
	// base is the HTTP origin requests go to; the printer URIs inside
	// them always name localhost, as libcups writes them.
	base string
	ids  atomic.Uint32
}

// requestTimeout bounds one request: CUPS answers at once, and a
// spooling document is local.
const requestTimeout = 30 * time.Second

// NewClient addresses server: an absolute socket path or host[:port].
func NewClient(server string) *Client {
	c := &Client{http: &http.Client{Timeout: requestTimeout}}
	if strings.HasPrefix(server, "/") {
		path := server
		c.http.Transport = &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}}
		c.base = "http://localhost"
		return c
	}
	if !strings.Contains(server, ":") {
		server += ":631"
	}
	c.base = "http://" + server
	return c
}

// Printer is one queue as the dialog lists it, with what it supports
// (empty when it does not say).
type Printer struct {
	Name, Location, StateMessage string
	// State is printer-state: 3 idle, 4 processing, 5 stopped.
	State     int32
	Accepting bool
	// UUID is its printer-uuid ("urn:uuid:…"), DeviceURI where CUPS
	// sends its jobs.
	UUID, DeviceURI string
	// StateReasons are its printer-state-reasons keywords ("none" left
	// out), Markers its supplies.
	StateReasons []string
	Markers      []Marker
	// Media are its PWG media names, Sides its sides keywords,
	// ColorModes its print-color-mode keywords, Qualities its
	// print-quality enums; each with its default.
	Media, Sides, ColorModes   []string
	MediaDefault, SidesDefault string
	ColorModeDefault           string
	Qualities                  []int32
	QualityDefault             int32
	// NumberUp are the pages per sheet it can print (number-up-
	// supported, its ranges spelled out up to maxNumberUp), Scalings its
	// print-scaling keywords, MediaSources its trays, MediaTypes its
	// media types and OutputBins its output trays; each with its default
	// where IPP has one.
	NumberUp                 []int32
	Scalings                 []string
	ScalingDefault           string
	MediaSources, MediaTypes []string
	OutputBins               []string
	OutputBinDefault         string
}

// maxNumberUp bounds a number-up-supported range spelled out.
const maxNumberUp = 16

// Marker is one supply (marker-names, marker-colors, marker-levels):
// its level in percent, negative when the printer cannot tell.
type Marker struct {
	Name, Color string
	Level       int32
}

// The printer-state values.
const (
	StateIdle       int32 = 3
	StateProcessing int32 = 4
	StateStopped    int32 = 5
)

// printerAttributes are the attributes a Printer reads.
var printerAttributes = []string{
	"printer-name", "printer-location", "printer-state", "printer-state-message", "printer-state-reasons",
	"printer-is-accepting-jobs", "printer-uuid", "device-uri", "marker-names", "marker-colors", "marker-levels",
	"media-supported", "media-default", "sides-supported", "sides-default",
	"print-color-mode-supported", "print-color-mode-default", "print-quality-supported", "print-quality-default",
	"number-up-supported", "print-scaling-supported", "print-scaling-default", "media-source-supported",
	"media-type-supported", "output-bin-supported", "output-bin-default",
}

// operation is the operation group every request opens with.
func operation(printerURI string, extra ...Attr) Group {
	attrs := []Attr{
		String(TagCharset, "attributes-charset", "utf-8"),
		String(TagLanguage, "attributes-natural-language", "en"),
		String(TagURI, "printer-uri", printerURI),
	}
	return Group{Tag: GroupOperation, Attrs: append(attrs, extra...)}
}

// PrinterURI is the queue's URI as CUPS names it.
func PrinterURI(name string) string { return "ipp://localhost/printers/" + url.PathEscape(name) }

// Printers lists the queues (CUPS-Get-Printers).
func (c *Client) Printers(ctx context.Context) ([]Printer, error) {
	req := Message{Code: OpCUPSGetPrinters, Groups: []Group{operation("ipp://localhost/",
		String(TagKeyword, "requested-attributes", printerAttributes...),
	)}}
	resp, err := c.do(ctx, "/", req, nil)
	if err != nil {
		return nil, err
	}
	var out []Printer
	for _, g := range resp.Groups {
		if p, ok := printerOf(g); ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// Printer reads one queue's attributes (Get-Printer-Attributes).
func (c *Client) Printer(ctx context.Context, name string) (Printer, error) {
	return c.Attributes(ctx, PrinterURI(name))
}

// Attributes reads the attributes of the printer at printerURI, posted
// to its path (Get-Printer-Attributes).
func (c *Client) Attributes(ctx context.Context, printerURI string) (Printer, error) {
	u, err := url.Parse(printerURI)
	if err != nil {
		return Printer{}, fmt.Errorf("ipp: printer URI %q: %w", printerURI, err)
	}
	req := Message{Code: OpGetPrinterAttributes, Groups: []Group{operation(printerURI,
		String(TagKeyword, "requested-attributes", printerAttributes...),
	)}}
	resp, err := c.do(ctx, u.EscapedPath(), req, nil)
	if err != nil {
		return Printer{}, err
	}
	for _, g := range resp.Groups {
		if p, ok := printerOf(g); ok {
			return p, nil
		}
	}
	return Printer{}, fmt.Errorf("ipp: no attributes for printer %s", printerURI)
}

// CreateLocalPrinter has CUPS make a temporary IPP Everywhere queue for
// the printer at deviceURI, which it removes once idle
// (CUPS-Create-Local-Printer), and reports its name.
func (c *Client) CreateLocalPrinter(ctx context.Context, deviceURI, name string) (string, error) {
	req := Message{Code: OpCUPSCreateLocalPrinter, Groups: []Group{
		operation("ipp://localhost/"),
		{Tag: GroupPrinter, Attrs: []Attr{String(TagName, "printer-name", name), String(TagURI, "device-uri", deviceURI)}},
	}}
	resp, err := c.do(ctx, "/", req, nil)
	if err != nil {
		return "", fmt.Errorf("ipp: a queue for %s: %w", deviceURI, err)
	}
	for _, g := range resp.Groups {
		if g.Tag != GroupPrinter {
			continue
		}
		u, err := url.Parse(g.Text("printer-uri-supported"))
		if err != nil {
			continue
		}
		if queue, ok := strings.CutPrefix(u.Path, "/printers/"); ok && queue != "" {
			return url.PathUnescape(queue)
		}
	}
	return "", fmt.Errorf("ipp: CUPS named no queue for %s", deviceURI)
}

// DeviceAttributes is a reader of the attributes of a network printer
// at its own ipp:// or ipps:// URI, as the dialog does for one found
// over DNS-SD before CUPS has a queue for it. An ipps printer whose
// certificate chains to a system root for its host is trusted as such;
// any other certificate valid in itself is trusted on first use and
// pinned in a file under pinDir named after the printer's host and
// port, and a later certificate that differs is refused with
// ErrCertificateChanged.
func DeviceAttributes(pinDir string) func(ctx context.Context, uri string) (Printer, error) {
	return (&devices{pins: pinDir}).attributes
}

// ErrCertificateChanged is an ipps printer presenting a certificate
// other than the one pinned on first contact.
var ErrCertificateChanged = errors.New("ipp: the printer's certificate changed since it was first trusted")

type devices struct {
	pins  string
	roots *x509.CertPool
	mu    sync.Mutex
}

func (d *devices) attributes(ctx context.Context, uri string) (Printer, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return Printer{}, fmt.Errorf("ipp: printer URI %q: %w", uri, err)
	}
	host := net.JoinHostPort(u.Hostname(), cmp.Or(u.Port(), "631"))
	c := &Client{http: &http.Client{Timeout: requestTimeout}}
	switch u.Scheme {
	case "ipp":
		c.base = (&url.URL{Scheme: "http", Host: host}).String()
	case "ipps":
		c.base = (&url.URL{Scheme: "https", Host: host}).String()
		c.http.Transport = &http.Transport{DialTLSContext: d.dialTLS}
	default:
		return Printer{}, fmt.Errorf("ipp: printer URI %q is neither ipp nor ipps", uri)
	}
	defer c.http.CloseIdleConnections()
	return c.Attributes(ctx, uri)
}

func (d *devices) dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	pinned, err := d.pinned(addr)
	if err != nil {
		return nil, err
	}
	if pinned != nil {
		conn, err := dialPinned(ctx, network, addr, pinned)
		if !isVerification(err) {
			return conn, err
		}
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ipp: printer address %q: %w", addr, err)
	}
	conn, err := dial(ctx, network, addr, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host, RootCAs: d.roots})
	var unverified *tls.CertificateVerificationError
	if !errors.As(err, &unverified) || len(unverified.UnverifiedCertificates) == 0 {
		return conn, err
	}
	presented := unverified.UnverifiedCertificates[0]
	if pinned != nil && !presented.Equal(pinned) {
		return nil, d.changed(addr, presented, pinned)
	}
	conn, err = dialPinned(ctx, network, addr, presented)
	if err != nil || pinned != nil {
		return conn, err
	}
	if err := d.pin(addr, presented); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func dialPinned(ctx context.Context, network, addr string, cert *x509.Certificate) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ipp: printer address %q: %w", addr, err)
	}
	name, err := certifiedName(cert, host)
	if err != nil {
		return nil, fmt.Errorf("ipp: the certificate of the printer at %s: %w", addr, err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return dial(ctx, network, addr, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: name, RootCAs: roots})
}

func certifiedName(cert *x509.Certificate, host string) (string, error) {
	switch {
	case cert.VerifyHostname(host) == nil:
		return host, nil
	case len(cert.DNSNames) > 0:
		return cert.DNSNames[0], nil
	case len(cert.IPAddresses) > 0:
		return cert.IPAddresses[0].String(), nil
	}
	return "", errors.New("it names no host or address to pin it to")
}

func dial(ctx context.Context, network, addr string, config *tls.Config) (net.Conn, error) {
	conn, err := (&tls.Dialer{Config: config}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("ipp: TLS to the printer at %s: %w", addr, err)
	}
	return conn, nil
}

func isVerification(err error) bool {
	var unverified *tls.CertificateVerificationError
	return errors.As(err, &unverified)
}

func fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

func (d *devices) pinPath(addr string) string { return filepath.Join(d.pins, url.PathEscape(addr)) }

func (d *devices) pinned(addr string) (*x509.Certificate, error) {
	if err := os.MkdirAll(d.pins, 0o700); err != nil {
		return nil, fmt.Errorf("ipp: the pinned certificate of %s: %w", addr, err)
	}
	root, err := os.OpenRoot(d.pins)
	if err != nil {
		return nil, fmt.Errorf("ipp: the pinned certificate of %s: %w", addr, err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(url.PathEscape(addr))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ipp: reading the pinned certificate of %s: %w", addr, err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("ipp: %s holds no PEM certificate", d.pinPath(addr))
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ipp: the pinned certificate in %s: %w", d.pinPath(addr), err)
	}
	return cert, nil
}

func (d *devices) changed(addr string, presented, pinned *x509.Certificate) error {
	return fmt.Errorf("%w: the printer at %s presents sha256:%s, pinned sha256:%s in %s; remove that file to trust the new one",
		ErrCertificateChanged, addr, fingerprint(presented), fingerprint(pinned), d.pinPath(addr))
}

func (d *devices) pin(addr string, cert *x509.Certificate) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	pinned, err := d.pinned(addr)
	switch {
	case err != nil:
		return err
	case pinned == nil:
	case pinned.Equal(cert):
		return nil
	default:
		return d.changed(addr, cert, pinned)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := atomicfile.WriteFile(d.pinPath(addr), data, 0o600); err != nil {
		return fmt.Errorf("ipp: pinning the certificate of %s: %w", addr, err)
	}
	return nil
}

// Default is the scheduler's default destination (CUPS-Get-Default),
// "" when it has none.
func (c *Client) Default(ctx context.Context) (string, error) {
	req := Message{Code: OpCUPSGetDefault, Groups: []Group{operation("ipp://localhost/",
		String(TagKeyword, "requested-attributes", "printer-name"),
	)}}
	resp, err := c.do(ctx, "/", req, nil)
	if IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, g := range resp.Groups {
		if g.Tag == GroupPrinter {
			return g.Text("printer-name"), nil
		}
	}
	return "", nil
}

// printerOf reads a printer group; false for another group or a
// nameless printer.
func printerOf(g Group) (Printer, bool) {
	if g.Tag != GroupPrinter {
		return Printer{}, false
	}
	p := Printer{
		Name: g.Text("printer-name"), Location: g.Text("printer-location"), StateMessage: g.Text("printer-state-message"),
		UUID: g.Text("printer-uuid"), DeviceURI: g.Text("device-uri"), Accepting: true,
	}
	if p.Name == "" {
		return Printer{}, false
	}
	if s, ok := g.Int("printer-state"); ok {
		p.State = s
	}
	if a, ok := g.Int("printer-is-accepting-jobs"); ok {
		p.Accepting = a != 0
	}
	for _, r := range g.Texts("printer-state-reasons") {
		if r != "none" {
			p.StateReasons = append(p.StateReasons, r)
		}
	}
	names, colors, levels := g.Texts("marker-names"), g.Texts("marker-colors"), g.Ints("marker-levels")
	for i, name := range names {
		m := Marker{Name: name, Level: -1}
		if i < len(colors) {
			m.Color = colors[i]
		}
		if i < len(levels) {
			m.Level = levels[i]
		}
		p.Markers = append(p.Markers, m)
	}
	p.Media, p.MediaDefault = g.Texts("media-supported"), g.Text("media-default")
	p.Sides, p.SidesDefault = g.Texts("sides-supported"), g.Text("sides-default")
	p.ColorModes, p.ColorModeDefault = g.Texts("print-color-mode-supported"), g.Text("print-color-mode-default")
	p.Qualities = g.Ints("print-quality-supported")
	p.QualityDefault, _ = g.Int("print-quality-default")
	p.NumberUp = numberUp(g)
	p.Scalings, p.ScalingDefault = g.Texts("print-scaling-supported"), g.Text("print-scaling-default")
	p.MediaSources, p.MediaTypes = g.Texts("media-source-supported"), g.Texts("media-type-supported")
	p.OutputBins, p.OutputBinDefault = g.Texts("output-bin-supported"), g.Text("output-bin-default")
	return p, true
}

// numberUp spells number-up-supported out: its integers, and its
// ranges up to maxNumberUp.
func numberUp(g Group) []int32 {
	a, _ := g.Get("number-up-supported")
	var out []int32
	for _, v := range a.Values {
		if v.Tag != TagRange {
			out = append(out, v.Int)
			continue
		}
		for n := max(v.Low, 1); n <= min(v.Int, maxNumberUp); n++ {
			out = append(out, n)
		}
	}
	return out
}

// Job is one document to spool.
type Job struct {
	Printer, Title, User string
	// Format is the document's MIME type.
	Format string
	// Attrs are the job template attributes (copies, sides, media...).
	Attrs    []Attr
	Document io.Reader
}

// Print sends a job (Print-Job) and returns its id.
func (c *Client) Print(ctx context.Context, j Job) (int32, error) {
	groups := []Group{operation(PrinterURI(j.Printer),
		String(TagName, "requesting-user-name", j.User),
		String(TagName, "job-name", j.Title),
		String(TagMimeMedia, "document-format", j.Format),
	)}
	if len(j.Attrs) > 0 {
		groups = append(groups, Group{Tag: GroupJob, Attrs: j.Attrs})
	}
	resp, err := c.do(ctx, "/printers/"+url.PathEscape(j.Printer), Message{Code: OpPrintJob, Groups: groups}, j.Document)
	if err != nil {
		return 0, err
	}
	for _, g := range resp.Groups {
		if id, ok := g.Int("job-id"); ok {
			return id, nil
		}
	}
	return 0, nil
}

// do posts one request (and its document) and decodes the answer; an
// unsuccessful status is an error carrying status-message.
func (c *Client) do(ctx context.Context, path string, m Message, doc io.Reader) (Message, error) {
	m.RequestID = c.ids.Add(1)
	if err := m.checkLengths(); err != nil {
		return Message{}, err
	}
	body := io.Reader(bytes.NewReader(m.Encode()))
	if doc != nil {
		body = io.MultiReader(body, doc)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, body)
	if err != nil {
		return Message{}, err
	}
	req.Header.Set("Content-Type", "application/ipp")
	res, err := c.http.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("ipp: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return Message{}, fmt.Errorf("ipp: HTTP %s", res.Status)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return Message{}, fmt.Errorf("ipp: %w", err)
	}
	resp, err := Decode(data)
	if err != nil {
		return Message{}, err
	}
	if !StatusOK(resp.Code) {
		msg := ""
		for _, g := range resp.Groups {
			if t := g.Text("status-message"); t != "" {
				msg = t
			}
		}
		return resp, &StatusError{Code: resp.Code, Message: msg}
	}
	return resp, nil
}

// StatusError is an unsuccessful IPP status.
type StatusError struct {
	Code    uint16
	Message string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("ipp: status %#04x: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("ipp: status %#04x", e.Code)
}

// IsNotFound reports client-error-not-found (no such printer, or no
// printers at all for CUPS-Get-Printers).
func IsNotFound(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == 0x0406
}
