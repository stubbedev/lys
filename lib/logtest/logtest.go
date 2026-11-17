// Package logtest collects what the process log receives during a test.
package logtest

import (
	"bytes"
	"log"
	"sync"
	"testing"
)

// Log is the process log a test collected, safe to write from any goroutine.
type Log struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *Log) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *Log) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// Capture sends the process log to a Log until t ends, then back to where it went.
func Capture(t testing.TB) *Log {
	t.Helper()
	l := &Log{}
	out := log.Writer()
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(out) })
	return l
}
