package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const packagedPAM = "../../resources/pam"

var fingerprintStacks = map[string]bool{
	"common-auth":      true,
	"system-auth":      true,
	"fingerprint-auth": true,
}

func authModules(service string) ([]string, error) {
	var modules []string
	for n, line := range strings.Split(service, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if fields[0] == "@include" {
			if len(fields) != 2 {
				return nil, fmt.Errorf("line %d: %q includes no one service", n+1, line)
			}
			if fingerprintStacks[fields[1]] {
				return nil, fmt.Errorf("line %d: %q brings in the distribution's pam_fprintd", n+1, line)
			}
			continue
		}
		if fields[0] != "auth" {
			continue
		}
		if len(fields) < 3 {
			return nil, fmt.Errorf("line %d: %q names no module", n+1, line)
		}
		control, module := fields[1], fields[2]
		if strings.HasPrefix(control, "[") {
			end := slices.IndexFunc(fields, func(f string) bool { return strings.HasSuffix(f, "]") })
			if end < 0 || end+1 >= len(fields) {
				return nil, fmt.Errorf("line %d: %q has an unterminated control", n+1, line)
			}
			module = fields[end+1]
		}
		if (control == "include" || control == "substack") && fingerprintStacks[module] {
			return nil, fmt.Errorf("line %d: %q brings in the distribution's pam_fprintd", n+1, line)
		}
		if module == "pam_fprintd.so" {
			return nil, fmt.Errorf("line %d: %q waits for a scan before the password", n+1, line)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func packagedService(t *testing.T, distro, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(packagedPAM, distro, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheDebianPasswordStackHasNoFingerprint(t *testing.T) {
	modules, err := authModules(packagedService(t, "debian", "lys-login"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"pam_nologin.so", "pam_unix.so"}; !slices.Equal(modules, want) {
		t.Errorf("auth modules %q, want %q: the password is checked by pam_unix with nothing before it", modules, want)
	}
}

func TestTheFedoraPasswordStackHasNoFingerprint(t *testing.T) {
	modules, err := authModules(packagedService(t, "fedora", "lys-login"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"password-auth", "postlogin"}; !slices.Equal(modules, want) {
		t.Errorf("auth stacks %q, want %q: authselect keeps pam_fprintd out of password-auth", modules, want)
	}
}

func TestTheFingerprintServicesAuthenticateNothingAgain(t *testing.T) {
	for _, distro := range []string{"debian", "fedora"} {
		modules, err := authModules(packagedService(t, distro, "lys-login-fingerprint"))
		if err != nil {
			t.Fatalf("%s: %v", distro, err)
		}
		if want := []string{"pam_nologin.so", "pam_systemd_loadkey.so", "pam_permit.so"}; !slices.Equal(modules, want) {
			t.Errorf("%s: auth modules %q, want %q", distro, modules, want)
		}
	}
}

func TestAStackThatWaitsForAScanIsRefused(t *testing.T) {
	for _, service := range []string{
		"auth requisite pam_nologin.so\n@include common-auth\n",
		"auth substack system-auth\n",
		"auth include fingerprint-auth\n",
		"auth [success=done default=ignore] pam_fprintd.so max-tries=1\nauth required pam_unix.so\n",
		"auth sufficient pam_fprintd.so\n",
	} {
		if modules, err := authModules(service); err == nil {
			t.Errorf("%q passed with %q, want refused", service, modules)
		}
	}
	for _, service := range []string{"auth [success=ok default=ignore\n", "@include\n", "auth required\n"} {
		if _, err := authModules(service); err == nil {
			t.Errorf("the malformed %q = %v, want refused", service, err)
		}
	}
}
