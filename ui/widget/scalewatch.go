package widget

import "github.com/stubbedev/lys/lib/render"

// ScaleWatch is an invisible widget that tells the application the
// device scale its tree presents at, as an Icon learns it to decode
// ahead of its paint: OnScale runs on the loop with num device pixels
// per denom logical ones each time SetDeviceScale gives the tree a new
// one, and when the watch is arranged into a tree whose scale is known.
type ScaleWatch struct {
	node
	seen    uint32
	onScale func(num, denom int)
}

var _ scalePreparer = (*ScaleWatch)(nil)

// NewScaleWatch returns a watch calling onScale on every new scale.
func NewScaleWatch(onScale func(num, denom int)) *ScaleWatch {
	return &ScaleWatch{onScale: onScale}
}

// Measure wants no space.
func (s *ScaleWatch) Measure(Axis, int) Sizes {
	return FixedSizes(0)
}

// Arrange records r and reports the tree's scale if it is known.
func (s *ScaleWatch) Arrange(r render.Rect) {
	s.node.Arrange(r)
	if root := styleRootOf(&s.node); root.deviceScale != 0 {
		s.prepare(root.deviceScale, root)
	}
}

// Paint paints nothing.
func (s *ScaleWatch) Paint(*render.Canvas) {}

// HitTest returns nil: the watch is not interactive.
func (s *ScaleWatch) HitTest(Point) Widget { return nil }

func (s *ScaleWatch) prepare(frac uint32, _ *node) {
	if frac == s.seen {
		return
	}
	s.seen = frac
	s.onScale(int(frac), iconScaleDenom)
}
