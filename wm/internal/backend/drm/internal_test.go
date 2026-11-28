package drm

import (
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
)

func TestAPanelIsInternalByItsConnectorType(t *testing.T) {
	for _, c := range []struct {
		connector uint32
		name      string
		internal  bool
	}{
		{connectorLVDS, "LVDS-1", true},
		{connectorEDP, "eDP-1", true},
		{connectorDSI, "DSI-1", true},
		{10, "DP-1", false},
		{11, "HDMI-A-1", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := rigged(t, func(*fakeDevice) allocator { return &fakeAlloc{} }, HoldsMaster)
			r.dev.connectorType = c.connector
			if err := r.b.Start(r.rec); err != nil {
				t.Fatal(err)
			}
			o := r.b.Outputs()[0]
			var p backend.Panel = o
			if o.Name() != c.name || p.Internal() != c.internal {
				t.Fatalf("%s is internal: %v, want %s internal: %v", o.Name(), p.Internal(), c.name, c.internal)
			}
		})
	}
}
