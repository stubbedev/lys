package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckPAMRefusesBeforeStartingPAM(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no service", nil, 2, "want one PAM service, got 0"},
		{"two services", []string{"login", "su"}, 2, "want one PAM service, got 2"},
		{"a path", []string{"../shadow"}, 1, "not a service name"},
		{"a service no file defines", []string{"lys-login-test-no-such-service"}, 1, "no such file"},
	} {
		cmd := exec.Command(os.Args[0], append([]string{"check-pam"}, c.args...)...)
		cmd.Env = append(os.Environ(), "LYS_LOGIN_TEST_MAIN=1", "USER=alice")
		var stderr strings.Builder
		cmd.Stderr = &stderr
		_ = cmd.Run()
		if code := cmd.ProcessState.ExitCode(); code != c.code || !strings.Contains(stderr.String(), c.want) {
			t.Errorf("%s: exit %d, stderr %q; want %d and %q", c.name, code, stderr.String(), c.code, c.want)
		}
	}
}
