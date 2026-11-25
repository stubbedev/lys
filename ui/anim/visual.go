package anim

// Visual is what a transient surface does on one direction of its
// motion, enter or exit, beside its reveal.
type Visual uint8

const (
	// VisualFade only fades the surface.
	VisualFade Visual = iota
	// VisualGrow grows the surface out of the side facing its anchor
	// while it fades in, and shrinks it back on the way out.
	VisualGrow
	// VisualLift scales the surface about its center from
	// widget.LiftScale while it fades in, and back up to it while it
	// fades out: a dismissed surface lifts away toward the viewer.
	VisualLift
)
