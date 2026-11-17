// Package logtest collects what the process log receives during a test.
package logtest

import (
	"bytes"
	"io"
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

// Reset drops what the Log collected so far.
func (l *Log) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Reset()
}

// Capture sends the process log, without timestamps, to a Log until t ends,
// then back to where it went with the flags it had.
func Capture(t testing.TB) *Log {
	t.Helper()
	l := &Log{}
	redirect(t, l)
	return l
}

// Tee is Capture that still writes the process log where it went.
func Tee(t testing.TB) *Log {
	t.Helper()
	l := &Log{}
	redirect(t, io.MultiWriter(log.Writer(), l))
	return l
}

func redirect(t testing.TB, w io.Writer) {
	out, flags := log.Writer(), log.Flags()
	log.SetOutput(w)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(out)
		log.SetFlags(flags)
	})
}
