package auth

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func pamDirs(t *testing.T, services ...string) []string {
	t.Helper()
	etc, vendor := filepath.Join(t.TempDir(), "etc"), filepath.Join(t.TempDir(), "vendor")
	for _, dir := range []string{etc, vendor} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range services {
		if err := os.WriteFile(filepath.Join(vendor, s), []byte("auth required pam_unix.so\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return []string{etc, vendor}
}

func TestAServiceWhoseAccountStackAcceptsTheUserPassesWithoutAuthenticating(t *testing.T) {
	lib := &fakePAM{acctBatches: [][]pamMessage{{{style: pamTextInfo, text: "password expires in 9 days"}}}}
	if err := checkService(lib, pamDirs(t, "system-auth"), "system-auth", "alice"); err != nil {
		t.Fatalf("check = %v", err)
	}
	if lib.service != "system-auth" || lib.user != "alice" {
		t.Errorf("checked %s for %s", lib.service, lib.user)
	}
	if want := []string{"acct_mgmt", "end"}; !slices.Equal(lib.calls, want) {
		t.Errorf("the check called %q, want %q: never authenticate, so no failure is counted", lib.calls, want)
	}
}

func TestAServiceNoFileDefinesFailsBeforePAMStarts(t *testing.T) {
	lib := &fakePAM{}
	err := checkService(lib, pamDirs(t, "login"), "system-auth", "alice")
	if err == nil || !strings.Contains(err.Error(), "no such file") || !strings.Contains(err.Error(), `"other"`) {
		t.Fatalf("check of a missing service = %v", err)
	}
	if lib.service != "" {
		t.Error("PAM started for a service no file defines")
	}
}

func TestAServiceDirectoryIsNoService(t *testing.T) {
	dirs := pamDirs(t)
	if err := os.Mkdir(filepath.Join(dirs[0], "login"), 0o755); err != nil {
		t.Fatal(err)
	}
	lib := &fakePAM{}
	if err := checkService(lib, dirs, "login", "alice"); err == nil || !strings.Contains(err.Error(), "is a directory") || lib.service != "" {
		t.Fatalf("check of a directory = %v, PAM started %v", err, lib.service != "")
	}
}

func TestAServiceNameThatIsAPathIsRefused(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../shadow", "a/b"} {
		lib := &fakePAM{}
		if err := checkService(lib, pamDirs(t), name, "alice"); err == nil || lib.service != "" {
			t.Errorf("check of %q = %v, PAM started %v", name, err, lib.service != "")
		}
	}
}

func TestAnAccountTheStackRefusesSaysWhy(t *testing.T) {
	lib := &fakePAM{acctRC: fakeAcctErr}
	err := checkService(lib, pamDirs(t, "login"), "login", "alice")
	if err == nil || !strings.Contains(err.Error(), "User account has expired") {
		t.Fatalf("check of a refused account = %v", err)
	}
	if lib.endStatus != fakeAcctErr {
		t.Errorf("the transaction ended with %d, want the refusal", lib.endStatus)
	}
}

func TestAnAccountStackThatPromptsFails(t *testing.T) {
	lib := &fakePAM{acctBatches: [][]pamMessage{{{style: pamPromptEchoOff, text: "Password: "}}}}
	if err := checkService(lib, pamDirs(t, "login"), "login", "alice"); !errors.Is(err, errPromptedInCheck) {
		t.Fatalf("check of a prompting account stack = %v", err)
	}
}

func TestAFailedStartIsReported(t *testing.T) {
	lib := &fakePAM{startErr: errors.New("libpam.so.0: cannot open")}
	if err := checkService(lib, pamDirs(t, "login"), "login", "alice"); err == nil || !strings.Contains(err.Error(), "libpam.so.0") {
		t.Fatalf("check with no libpam = %v", err)
	}
}

func TestNoUserIsRefused(t *testing.T) {
	lib := &fakePAM{}
	if err := checkService(lib, pamDirs(t, "login"), "login", ""); err == nil || lib.service != "" {
		t.Fatalf("a check for no user = %v, PAM started %v", err, lib.service != "")
	}
}
