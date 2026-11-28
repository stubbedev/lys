package filebrowser

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/netfs"
	"github.com/stubbedev/lys/desktop/vfs/sftpfs/sftptest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/webdav"
)

// memKeyring is a keyring in memory.
type memKeyring struct {
	mu     sync.Mutex
	items  map[string][]byte
	stored int
}

func attrKey(attrs map[string]string) string { return attrs["xdg:schema"] + "|" + attrs["server"] }

func (k *memKeyring) Lookup(_ context.Context, attrs map[string]string) ([]byte, bool, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.items[attrKey(attrs)]
	return v, ok, nil
}

func (k *memKeyring) Store(_ context.Context, _ string, attrs map[string]string, value []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.items == nil {
		k.items = map[string][]byte{}
	}
	k.items[attrKey(attrs)] = value
	k.stored++
	return nil
}

// netRig is a browser with a network connecting to an in-process SFTP
// server serving bob, password pw, its known_hosts and its recent
// servers in scratch files.
type netRig struct {
	*harness
	sftp       *sftptest.Server
	knownHosts string
	recent     string
	keyring    *memKeyring
	share      string
}

func newNetRig(t *testing.T) *netRig {
	t.Helper()
	dir := t.TempDir()
	r := &netRig{sftp: sftptest.Start(t, "bob", "pw"), knownHosts: filepath.Join(dir, "ssh", "known_hosts"), recent: filepath.Join(dir, "servers"), keyring: &memKeyring{}}
	r.share = t.TempDir()
	for _, f := range []string{"report.txt", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(r.share, f), []byte(f), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	network := NewNetwork(func(ctx context.Context, loc vfs.Location, cred vfs.Credentials, trust *vfs.Trust) (vfs.Conn, error) {
		return netfs.Connect(ctx, loc, cred, trust, netfs.Options{})
	}, &vfs.Trust{KnownHosts: r.knownHosts}, r.keyring, r.recent)
	t.Cleanup(network.Close)
	r.harness = newHarnessWith(t, "", func(d *Deps) { d.Network = network })
	t.Cleanup(func() { _ = r.mounts.Close() })
	r.open(t, openFile)
	return r
}

// location is the share's location on the server for bob.
func (r *netRig) location() string { return r.sftp.Server("bob").String() + r.share }

// trustKey lists key for the server in known_hosts.
func (r *netRig) trustKey(t *testing.T, key ssh.PublicKey) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(r.knownHosts), 0o700); err != nil {
		t.Fatal(err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(r.sftp.Addr)}, key)
	if err := os.WriteFile(r.knownHosts, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// connectTo opens Connect to Server and connects to text.
func (r *netRig) connectTo(text string) {
	r.do(func() {
		r.c.ShowConnect(text)
		r.c.connectTyped(text)
	})
}

// connecting reports the band open.
func (r *netRig) connecting() bool {
	var open bool
	r.do(func() { open = r.c.Connecting() })
	return open
}

// note is what the band says.
func (r *netRig) note() string {
	var s string
	r.do(func() { s = r.c.ui.connect.note.Text() })
	return s
}

// logIn types the password and connects, remembered as how.
func (r *netRig) logIn(t *testing.T, password string, how Remember) {
	t.Helper()
	r.do(func() {
		p := r.c.ui.connect
		p.password.SetText(password)
		p.remember.SetChecked(slices.Index(p.choices, how), true)
		r.c.logIn()
	})
}

// locationLabels are the labels of the sidebar's Locations rows.
func (r *netRig) locationLabels() []string {
	var out []string
	r.do(func() {
		treetest.Walk(r.c.ui.locations, func(w widget.Widget) {
			if l, ok := w.(*widget.Label); ok {
				out = append(out, l.Text())
			}
		})
	})
	return out
}

// Connecting to a host never seen asks to trust its key, then for the
// password, and opens the share; the key lands in known_hosts, the
// server among the recent ones, and Locations lists it with a
// disconnect button. Disconnecting takes the row away and leaves it.
func TestConnectingOpensTheShareAndDisconnectingRemovesIt(t *testing.T) {
	r := newNetRig(t)
	loc := r.location()
	r.connectTo(loc)
	if want := i18n.T("file-chooser-host-key-unknown", i18n.Str("server", r.sftp.Server("bob").String()), i18n.Str("algorithm", r.sftp.Key.Type()), i18n.Str("fingerprint", ssh.FingerprintSHA256(r.sftp.Key))); r.note() != want {
		t.Fatalf("an unknown host says %q, want %q", r.note(), want)
	}
	r.click(t, i18n.T("file-chooser-button-trust"))
	if want := i18n.T("file-chooser-login", i18n.Str("server", r.sftp.Server("bob").String())); r.note() != want {
		t.Fatalf("after trusting the key the band says %q, want %q", r.note(), want)
	}
	if data, err := os.ReadFile(r.knownHosts); err != nil || !strings.Contains(string(data), strings.TrimSpace(string(ssh.MarshalAuthorizedKey(r.sftp.Key)))[len(r.sftp.Key.Type())+1:]) {
		t.Errorf("known_hosts holds %q, %v, want the server's key", data, err)
	}
	r.logIn(t, "pw", RememberSession)
	if got := r.dir(); got != loc {
		t.Fatalf("connecting opened %q, want %q", got, loc)
	}
	if got := r.shown(); !slices.Equal(got, []string{"notes.txt", "report.txt"}) {
		t.Errorf("the share lists %v", got)
	}
	label := r.sftp.Server("bob").String()[len("sftp://"):]
	if !slices.Contains(r.locationLabels(), label) || !r.hasButton(i18n.T("file-chooser-disconnect", i18n.Str("server", label))) {
		t.Fatalf("Locations shows %v, want %s with a disconnect button", r.locationLabels(), label)
	}
	waittest.Until(t, func() bool {
		data, _ := os.ReadFile(r.recent)
		return strings.TrimSpace(string(data)) == loc
	}, "the server to be kept among the recent ones")

	r.click(t, i18n.T("file-chooser-disconnect", i18n.Str("server", label)))
	if slices.Contains(r.locationLabels(), label) || r.hasButton(i18n.T("file-chooser-disconnect", i18n.Str("server", label))) {
		t.Errorf("after disconnecting Locations shows %v", r.locationLabels())
	}
	if got := r.dir(); got != r.root {
		t.Errorf("disconnecting left the browser on %q, want home", got)
	}
	if r.mounts.Mounted(r.sftp.Server("bob")) {
		t.Error("the server is still mounted")
	}

	logins := r.sftp.Logins()
	r.connectTo(loc)
	if got := r.dir(); got != loc || r.sftp.Logins() != logins+1 {
		t.Errorf("reconnecting with the session's password opened %q after %d logins", got, r.sftp.Logins()-logins)
	}
}

// A wrong password asks again, saying it was refused; the right one
// then connects, and a password kept in the keyring logs in without
// asking the next time.
func TestAWrongPasswordAsksAgain(t *testing.T) {
	r := newNetRig(t)
	r.trustKey(t, r.sftp.Key)
	r.connectTo(r.location())
	r.logIn(t, "wrong", RememberNever)
	if want := i18n.T("file-chooser-login-failed", i18n.Str("server", r.sftp.Server("bob").String())); r.note() != want {
		t.Fatalf("a wrong password says %q, want %q", r.note(), want)
	}
	if r.mounts.Mounted(r.sftp.Server("bob")) {
		t.Fatal("a wrong password mounted the server")
	}
	r.logIn(t, "pw", RememberKeyring)
	if got := r.dir(); got != r.location() {
		t.Fatalf("the right password opened %q", got)
	}
	waittest.Until(t, func() bool {
		r.keyring.mu.Lock()
		defer r.keyring.mu.Unlock()
		return r.keyring.stored == 1
	}, "the password to be stored in the keyring")
	r.do(func() { r.c.disconnect(r.mounts.List()[0]) })
	r.connectTo(r.location())
	if got := r.dir(); got != r.location() {
		t.Errorf("the keyring's password did not log in: the browser shows %q, the band says %q", got, r.note())
	}
}

// A host whose key changed is refused, saying so, and nothing is
// mounted; an unknown key declined mounts nothing and trusts nothing.
func TestARefusedHostKeyAborts(t *testing.T) {
	r := newNetRig(t)
	other := sftptest.Start(t, "bob", "pw")
	r.trustKey(t, other.Key)
	r.connectTo(r.location())
	if want := i18n.T("file-chooser-host-key-changed", i18n.Str("server", r.sftp.Server("bob").String())); r.note() != want {
		t.Fatalf("a changed key says %q, want %q", r.note(), want)
	}
	if r.hasButton(i18n.T("file-chooser-button-trust")) || r.mounts.Mounted(r.sftp.Server("bob")) || r.sftp.Logins() != 0 {
		t.Fatal("a changed key could be trusted or was logged in to")
	}

	if err := os.Remove(r.knownHosts); err != nil {
		t.Fatal(err)
	}
	r.connectTo(r.location())
	r.click(t, i18n.T("file-chooser-button-cancel"))
	if r.connecting() || r.mounts.Mounted(r.sftp.Server("bob")) {
		t.Error("declining an unknown key left the band open or mounted the server")
	}
	if _, err := os.Stat(r.knownHosts); !os.IsNotExist(err) {
		t.Errorf("declining an unknown key wrote known_hosts: %v", err)
	}
}

// A server nothing listens for says it cannot be reached; an address
// of no network location says so too, and the location entry takes an
// address to connect to.
func TestAnUnreachableServerSaysSo(t *testing.T) {
	r := newNetRig(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gone := vfs.Server{Scheme: vfs.SFTP, User: "bob", Host: "127.0.0.1", Port: uint16(ln.Addr().(*net.TCPAddr).Port)}
	_ = ln.Close()
	r.connectTo(gone.String() + "/x")
	if want := i18n.T("file-chooser-unreachable", i18n.Str("server", gone.String())); r.note() != want {
		t.Errorf("an unreachable server says %q, want %q", r.note(), want)
	}
	r.connectTo("http://example.com/")
	if want := i18n.T("file-chooser-location-invalid", i18n.Str("location", "http://example.com/")); r.note() != want {
		t.Errorf("an http address says %q, want %q", r.note(), want)
	}
	r.do(r.c.CloseConnect)
	r.trustKey(t, r.sftp.Key)
	r.do(func() { r.c.goToLocation(r.location()) })
	if !r.connecting() {
		t.Fatal("an address typed into the location entry opened no connection")
	}
	r.logIn(t, "pw", RememberNever)
	if got := r.dir(); got != r.location() {
		t.Errorf("the address typed opened %q", got)
	}
}

// A certificate that does not verify asks in the words of why: no
// authority vouches for it, it names another host, or it is out of its
// dates.
func TestCertificateQuestionsSayWhy(t *testing.T) {
	for want, err := range map[string]error{
		"file-chooser-certificate-untrusted":  x509.UnknownAuthorityError{},
		"file-chooser-certificate-wrong-host": x509.HostnameError{Host: "x"},
		"file-chooser-certificate-expired":    x509.CertificateInvalidError{Reason: x509.Expired},
	} {
		if got := certificateQuestion(fmt.Errorf("verify: %w", err)); got != want {
			t.Errorf("%T asks %s, want %s", err, got, want)
		}
	}
	if got := certificateQuestion(x509.CertificateInvalidError{Reason: x509.NotAuthorizedToSign}); got != "file-chooser-certificate-untrusted" {
		t.Errorf("a certificate not allowed to sign asks %s", got)
	}
}

// A davs:// server whose certificate no authority vouches for asks to
// trust it; once trusted, it asks for the password and opens.
func TestAnUntrustedCertificateIsAskedAbout(t *testing.T) {
	dav := &webdav.Handler{FileSystem: webdav.Dir(t.TempDir()), LockSystem: webdav.NewMemLS()}
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "bob" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		dav.ServeHTTP(w, r)
	}))
	t.Cleanup(s.Close)
	network := NewNetwork(func(ctx context.Context, loc vfs.Location, cred vfs.Credentials, trust *vfs.Trust) (vfs.Conn, error) {
		return netfs.Connect(ctx, loc, cred, trust, netfs.Options{})
	}, &vfs.Trust{Roots: x509.NewCertPool()}, nil, "")
	hs := newHarnessWith(t, "", func(d *Deps) { d.Network = network })
	t.Cleanup(func() { _ = hs.mounts.Close() })
	hs.open(t, openFile)
	r := &netRig{harness: hs}
	loc := "davs://bob@" + s.Listener.Addr().String() + "/"
	r.connectTo(loc)
	l, _ := vfs.Parse(loc)
	if want := i18n.T("file-chooser-certificate-untrusted", i18n.Str("server", l.Server.String()), i18n.Str("fingerprint", vfs.CertFingerprint(s.Certificate()))); r.note() != want {
		t.Fatalf("an untrusted certificate says %q, want %q", r.note(), want)
	}
	r.click(t, i18n.T("file-chooser-button-trust"))
	r.logIn(t, "pw", RememberNever)
	if got := r.dir(); got != loc {
		t.Errorf("trusting the certificate opened %q, the band saying %q", got, r.note())
	}
}
