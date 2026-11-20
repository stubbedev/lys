package config

// ScreenPosition is a spot on an output a surface sits at: a corner, or
// the middle of an edge.
//
// Screen position: a corner, or the middle of an edge.
type ScreenPosition string

// ScreenPosition values.
const (
	// Top-left corner.
	ScreenTopLeft ScreenPosition = "top-left"
	// Top-center edge.
	ScreenTopCenter ScreenPosition = "top-center"
	// Top-right corner.
	ScreenTopRight ScreenPosition = "top-right"
	// Bottom-left corner.
	ScreenBottomLeft ScreenPosition = "bottom-left"
	// Bottom-center edge.
	ScreenBottomCenter ScreenPosition = "bottom-center"
	// Bottom-right corner.
	ScreenBottomRight ScreenPosition = "bottom-right"
	// Center-left edge.
	ScreenCenterLeft ScreenPosition = "center-left"
	// Center-right edge.
	ScreenCenterRight ScreenPosition = "center-right"
)

var _ = registerEnum(ScreenTopLeft, ScreenTopCenter, ScreenTopRight, ScreenBottomLeft, ScreenBottomCenter, ScreenBottomRight, ScreenCenterLeft, ScreenCenterRight)
