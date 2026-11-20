package fdtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestAnExecutableRunsWhileOtherProcessesStart(t *testing.T) {
	dir := t.TempDir()
	stop := make(chan struct{})
	var forks sync.WaitGroup
	for range 4 {
		forks.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = exec.Command("true").Run()
			}
		})
	}
	defer func() {
		close(stop)
		forks.Wait()
	}()
	for i := range 200 {
		path := filepath.Join(dir, "run"+strings.Repeat("x", i%7)+string(rune('a'+i%26)))
		WriteExecutable(t, path, []byte("#!/bin/sh\necho ok\n"))
		out, err := exec.Command(path).Output()
		if err != nil || strings.TrimSpace(string(out)) != "ok" {
			t.Fatalf("run %d of %s: %q, %v", i, path, out, err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAnExecutableThatCannotBeWrittenFailsTheTest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "run")
	r := &fatalRecorder{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		WriteExecutable(r, path, []byte("#!/bin/sh\n"))
	}()
	<-done
	if !strings.Contains(r.failure, path) {
		t.Fatalf("writing into a missing directory failed with %q, want the path named", r.failure)
	}
}
