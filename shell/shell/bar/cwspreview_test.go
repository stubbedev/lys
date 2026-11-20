package bar

import (
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
)

func previewedCws(t *testing.T, previews bool) *cwsModule {
	t.Helper()
	cfg := config.DefaultsWorkspaces()
	cfg.WindowPreviews = previews
	f := newFakeCws([]barview.Workspace{occupiedWS(1, 1, "DP-1")}, []cwsWindow{{id: 7, workspace: 1, hasWorkspace: true}, {id: 8, workspace: 1, hasWorkspace: true}})
	return newTestCws(t, cfg.View(compositor.Niri), f)
}

func TestByDefaultRestingOnAWorkspaceOpensNoPreviewAndCapturesNothing(t *testing.T) {
	m := previewedCws(t, false)
	if m.preview != nil || m.previewOf != nil {
		t.Fatal("the default workspaces module built a window preview")
	}
	onHeadlessLoop(func() bool {
		m.Attach(fakeHost{})
		m.hoverSegment(0, m.strip.Segments().Segment(0), true)
		m.hoverSegment(0, m.strip.Segments().Segment(0), false)
		return true
	})
}

func TestWithWindowPreviewsRestingOnAWorkspacePreviewsItsWindows(t *testing.T) {
	m := previewedCws(t, true)
	if m.preview == nil {
		t.Fatal("window-previews: true built no window preview")
	}
	r := rigPreview(m.preview)
	m.Attach(fakeHost{})
	onHeadlessLoop(func() bool {
		m.hoverSegment(0, m.strip.Segments().Segment(0), true)
		return true
	})
	r.fire()
	if r.opened != 1 || m.preview.captures != 2 {
		t.Errorf("a rest opened %d previews capturing %d windows, want 1 capturing both of the workspace's", r.opened, m.preview.captures)
	}
	m.hoverSegment(0, m.strip.Segments().Segment(0), false)
	r.fire()
	if r.dismiss != 1 {
		t.Errorf("leaving dismissed %d previews, want 1", r.dismiss)
	}
}
