// Package wallclocktest sets the time zone a wallclock.SystemClock under test
// watches.
package wallclocktest

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stubbedev/lys/lib/intconv"
)

// SetZone replaces the zone file at path, as timedated does, with a zone named
// name that is offset seconds east of UTC.
func SetZone(tb testing.TB, path, name string, offset int32) {
	tb.Helper()
	staged := path + ".new"
	if err := os.WriteFile(staged, tzif(name, offset), 0o600); err != nil {
		tb.Fatalf("stage the time zone %s for %s: %v", name, path, err)
	}
	if err := os.Rename(staged, path); err != nil {
		tb.Fatalf("set the time zone %s in %s: %v", name, path, err)
	}
}

func tzif(name string, offset int32) []byte {
	var b bytes.Buffer
	b.WriteString("TZif")
	b.Write(make([]byte, 16))
	for _, n := range []uint32{0, 0, 0, 0, 1, intconv.Must[uint32](len(name) + 1)} {
		_ = binary.Write(&b, binary.BigEndian, n)
	}
	_ = binary.Write(&b, binary.BigEndian, offset)
	b.Write([]byte{0, 0})
	b.WriteString(name)
	b.WriteByte(0)
	return b.Bytes()
}
