package powersupply

import (
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/udevmonitor"
	"github.com/stubbedev/lys/lib/udevmonitor/udevmonitortest"
)

// Groups stands udevmonitortest groups in for udevd, one per listener.
type Groups struct {
	t      testing.TB
	mu     sync.Mutex
	groups []*udevmonitortest.Group
}

func (g *Groups) listen(subsystems []string) (*udevmonitor.Listener, error) {
	group := udevmonitortest.New(g.t)
	g.mu.Lock()
	g.groups = append(g.groups, group)
	g.mu.Unlock()
	return group.Listen(subsystems)
}

// Send broadcasts the event to every listener.
func (g *Groups) Send(props ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, group := range g.groups {
		group.Send(props...)
	}
}

// NewTest is New(dir) on test groups, refreshing every refresh.
func NewTest(t testing.TB, dir string, refresh time.Duration) (*Sysfs, *Groups) {
	g := &Groups{t: t}
	s := New(dir)
	s.listen, s.refresh = g.listen, refresh
	return s, g
}
