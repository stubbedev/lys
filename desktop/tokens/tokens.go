// Package tokens is the one definition of the design every lys surface
// draws with: the default palette, the type scale, spacing, radii and the
// field. It imports nothing, so the boot splash embeds the same values
// the desktop's stylesheet, the greeter and the lock screen resolve.
package tokens

// Palette is a theme's ten colors as #rrggbb strings.
type Palette struct {
	Bg, Surface, Elevated string
	Fg, FgMuted           string
	Primary               string
	Red, Yellow           string
	Green, Blue           string
}

// LysShell is the default palette.
func LysShell() Palette {
	return Palette{
		Bg: "#141420", Surface: "#1c1c2c", Elevated: "#262638",
		Fg: "#d4d6e8", FgMuted: "#8a8ca4", Primary: "#e0947a",
		Red: "#e46870", Yellow: "#e0b870", Green: "#68c898", Blue: "#78a0e0",
	}
}

// RemPx is the pixels of 1rem, the body text size every rem token
// multiplies.
const RemPx = 16.0

// TextSize is a step of the type scale as a ratio of body.
type TextSize float64

// The type scale, smallest first.
const (
	Caption  TextSize = 0.875
	Body     TextSize = 1
	Subtitle TextSize = 1.125
	Title    TextSize = 1.375
	Display  TextSize = 2.25
	Hero     TextSize = 4
	Poster   TextSize = 7
)

// Px is the size in pixels over a body of rem pixels at scale.
func (s TextSize) Px(rem, scale float64) float64 { return rem * float64(s) * scale }

// Rem is a length in rem.
type Rem float64

// Px is the length in pixels over a rem of rem pixels at scale.
func (r Rem) Px(rem, scale float64) float64 { return float64(r) * rem * scale }

// The spacing scale.
const (
	Space2XS Rem = 0.125
	SpaceXS  Rem = 0.25
	SpaceSM  Rem = 0.5
	SpaceMD  Rem = 1
	SpaceLG  Rem = 1.5
	SpaceXL  Rem = 2
)

// The corner radius scale.
const (
	RadiusSM Rem = 0.375
	RadiusMD Rem = 0.625
	RadiusLG Rem = 0.875
	RadiusXL Rem = 1.125
)

// RadiusFullPx is the radius of a pill: larger than any side, so the ends
// are half circles.
const RadiusFullPx = 9999

// Percent is a share of a color mixed into the surface under it.
type Percent int

// The field, the pill an entry and the login screen's buttons rest as:
// the text color at FieldFill over the surface, ringed inside by the text
// color at FieldBorder, with RadiusFullPx corners.
const (
	FieldFill   Percent = 16
	FieldBorder Percent = 12
)
