package main

import (
	"strings"
	"testing"
)

func TestTheDaemonizedVariableNamesTheLogTarget(t *testing.T) {
	for _, target := range []logTarget{logsToStderr, logsToKmsg} {
		got, daemonized, err := daemonizedAs(target.String())
		if err != nil || !daemonized || got != target {
			t.Errorf("daemonizedAs(%q) = %v, %v, %v; want %v, daemonized", target, got, daemonized, err, target)
		}
	}
	if got, daemonized, err := daemonizedAs(""); err != nil || daemonized || got != logsToStderr {
		t.Errorf("daemonizedAs unset = %v, %v, %v; want stderr, not daemonized", got, daemonized, err)
	}
}

func TestADaemonizedVariableNamingNoTargetIsAnError(t *testing.T) {
	for _, value := range []string{"1", "yes", "KMSG", " kmsg"} {
		if _, daemonized, err := daemonizedAs(value); err == nil || daemonized || !strings.Contains(err.Error(), daemonizedEnv) {
			t.Errorf("daemonizedAs(%q) = %v, %v; want an error naming %s", value, daemonized, err, daemonizedEnv)
		}
	}
}
