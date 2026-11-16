package ipp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type certSpec struct {
	parent    *x509.Certificate
	parentKey *ecdsa.PrivateKey
	ca        bool
	notBefore time.Time
	notAfter  time.Time
	usage     []x509.ExtKeyUsage
}

func validSpec() certSpec {
	return certSpec{notBefore: time.Now().Add(-time.Hour), notAfter: time.Now().Add(time.Hour), usage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
}

func makeCert(t *testing.T, spec certSpec) (tls.Certificate, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "printer"},
		NotBefore:    spec.notBefore, NotAfter: spec.notAfter,
		ExtKeyUsage:           spec.usage,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		BasicConstraintsValid: true, IsCA: spec.ca,
	}
	parent, parentKey := tmpl, key
	if spec.parent != nil {
		parent, parentKey = spec.parent, spec.parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, leaf, key
}

type tlsPrinter struct {
	uri  string
	cert atomic.Pointer[tls.Certificate]
}

func answerAttributes(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	req, err := Decode(body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	resp := Message{RequestID: req.RequestID, Groups: []Group{operation(""), {Tag: GroupPrinter, Attrs: []Attr{String(TagName, "printer-name", "laser")}}}}
	_, _ = w.Write(resp.Encode())
}

func startTLSPrinter(t *testing.T, cert tls.Certificate) *tlsPrinter {
	t.Helper()
	p := &tlsPrinter{}
	p.cert.Store(&cert)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(answerAttributes))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*p.cert.Load()}}, nil
	}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	p.uri = "ipps://" + u.Host + "/ipp/print"
	return p
}

func pinFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return entries
}

func TestAnUntrustedCertificateIsPinnedOnFirstUseAndTrustedAfter(t *testing.T) {
	cert, _, _ := makeCert(t, validSpec())
	p := startTLSPrinter(t, cert)
	dir := filepath.Join(t.TempDir(), "pins")
	if _, err := DeviceAttributes(dir)(t.Context(), p.uri); err != nil {
		t.Fatalf("first contact: %v", err)
	}
	pins := pinFiles(t, dir)
	if len(pins) != 1 {
		t.Fatalf("%d pin files after first contact, want 1", len(pins))
	}
	if info, err := pins[0].Info(); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the pin file is %v, %v", info, err)
	}
	if _, err := DeviceAttributes(dir)(t.Context(), p.uri); err != nil {
		t.Errorf("a later contact with the same certificate: %v", err)
	}
	renewed, _, _ := makeCert(t, validSpec())
	p.cert.Store(&renewed)
	_, err := DeviceAttributes(dir)(t.Context(), p.uri)
	if !errors.Is(err, ErrCertificateChanged) {
		t.Errorf("a changed certificate: %v, want ErrCertificateChanged", err)
	}
	if len(pinFiles(t, dir)) != 1 {
		t.Error("the changed certificate replaced the pin")
	}
}

func TestACertificateChainingToARootVerifiesWithoutAPin(t *testing.T) {
	caSpec := validSpec()
	caSpec.ca = true
	_, ca, caKey := makeCert(t, caSpec)
	leafSpec := validSpec()
	leafSpec.parent, leafSpec.parentKey = ca, caKey
	leaf, _, _ := makeCert(t, leafSpec)
	p := startTLSPrinter(t, leaf)
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	dir := filepath.Join(t.TempDir(), "pins")
	d := &devices{pins: dir, roots: roots}
	if _, err := d.attributes(t.Context(), p.uri); err != nil {
		t.Fatalf("a CA-signed certificate: %v", err)
	}
	if n := len(pinFiles(t, dir)); n != 0 {
		t.Errorf("%d pin files for a CA-signed certificate, want none", n)
	}
	other, _, _ := makeCert(t, leafSpec)
	p.cert.Store(&other)
	if _, err := d.attributes(t.Context(), p.uri); err != nil {
		t.Errorf("another CA-signed certificate: %v", err)
	}
}

func TestAnInvalidCertificateIsRefusedAndNotPinned(t *testing.T) {
	expired := validSpec()
	expired.notBefore, expired.notAfter = time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	early := validSpec()
	early.notBefore, early.notAfter = time.Now().Add(time.Hour), time.Now().Add(2*time.Hour)
	clientOnly := validSpec()
	clientOnly.usage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	for name, spec := range map[string]certSpec{"expired": expired, "not yet valid": early, "client auth only": clientOnly} {
		t.Run(name, func(t *testing.T) {
			cert, _, _ := makeCert(t, spec)
			p := startTLSPrinter(t, cert)
			dir := filepath.Join(t.TempDir(), "pins")
			_, err := DeviceAttributes(dir)(t.Context(), p.uri)
			if err == nil || errors.Is(err, ErrCertificateChanged) {
				t.Errorf("an invalid certificate: %v, want a verification error", err)
			}
			if n := len(pinFiles(t, dir)); n != 0 {
				t.Errorf("%d pin files for an invalid certificate, want none", n)
			}
		})
	}
}
