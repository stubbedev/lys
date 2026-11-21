package closing

import (
	"errors"
	"testing"
)

type closer struct {
	err    error
	closed bool
}

func (c *closer) Close() error {
	c.closed = true
	return c.err
}

func TestJoinNamesWhatFailedToClose(t *testing.T) {
	t.Parallel()
	running := errors.New("the service stopped")
	failed := errors.New("broken pipe")
	c := &closer{err: failed}
	err := running
	Join(&err, "session bus", c)
	if !c.closed || !errors.Is(err, running) || !errors.Is(err, failed) || err.Error() != "the service stopped\nclose the session bus: broken pipe" {
		t.Errorf("Join = %q, closed %v", err, c.closed)
	}
}

func TestACleanCloseAddsNoError(t *testing.T) {
	t.Parallel()
	c := &closer{}
	var err error
	Join(&err, "system bus", c)
	if !c.closed || err != nil {
		t.Errorf("Join = %v, closed %v, want nil after a clean close", err, c.closed)
	}
}
