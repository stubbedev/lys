package icons

import (
	"fmt"
	"strings"

	"github.com/stubbedev/lys/desktop/icons/skpath"
	"github.com/stubbedev/lys/desktop/icons/usvg"
)

const (
	targetSize float32 = 16
	// resolutionScale is the stroker/dasher precision target.
	resolutionScale float32 = 4
	// FormatVersion is stamped onto every emitted SVG via gpa:version.
	FormatVersion = 3
)

type scaledPath struct {
	d             string
	isBoundingBox bool
}

type sourcePath struct {
	d      string
	stroke *skpath.Stroke
}

type symbolicPaths struct {
	filled []scaledPath
	source []sourcePath
}

// ToSymbolic converts svg to a symbolic icon; false when nothing can
// be extracted.
func ToSymbolic(svg string) (string, bool) {
	tree, err := usvg.Parse(svg)
	if err != nil {
		return buildFallbackSVG(svg)
	}
	sourceSize := max(tree.Width, tree.Height)
	scale := float32(1)
	if sourceSize > 0 {
		scale = targetSize / sourceSize
	}
	var paths symbolicPaths
	collectPaths(tree.Root, skpath.FromScale(scale, scale), &paths)
	if len(paths.filled) == 0 {
		return buildFallbackSVG(svg)
	}
	return buildGTKSVG(paths), true
}

func collectPaths(g *usvg.Group, parent skpath.Transform, paths *symbolicPaths) {
	for _, child := range g.Children {
		switch c := child.(type) {
		case *usvg.Path:
			processPath(c, parent, paths)
		case *usvg.Group:
			collectPaths(c, parent.PreConcat(c.Transform), paths)
		}
	}
}

func processPath(p *usvg.Path, parent skpath.Transform, paths *symbolicPaths) {
	transformed, ok := p.Data.Transform(parent)
	if !ok {
		return
	}
	d := pathData(transformed)
	if p.HasFill {
		pushPath(transformed, false, paths)
		paths.pushSource(sourcePath{d: d})
	}
	if p.Stroke != nil {
		if centre, st, ok := strokeCentreline(transformed, p.Stroke, parent); ok {
			if outlined, ok := centre.StrokePath(st, resolutionScale); ok {
				pushPath(outlined, false, paths)
				paths.pushSource(sourcePath{d: pathData(centre), stroke: &st})
			}
		}
	}
	if !p.HasFill && p.Stroke == nil {
		bbox := isBoundingBoxPath(transformed, targetSize)
		pushPath(transformed, bbox, paths)
		if !bbox {
			paths.pushSource(sourcePath{d: d})
		}
	}
}

func pushPath(p *skpath.Path, bbox bool, paths *symbolicPaths) {
	if d := pathData(p); d != "" {
		paths.filled = append(paths.filled, scaledPath{d: d, isBoundingBox: bbox})
	}
}

func (paths *symbolicPaths) pushSource(p sourcePath) {
	if p.d != "" {
		paths.source = append(paths.source, p)
	}
}

func (paths *symbolicPaths) stroked() bool {
	for _, p := range paths.source {
		if p.stroke != nil {
			return true
		}
	}
	return false
}

func strokeCentreline(geometry *skpath.Path, s *usvg.Stroke, parent skpath.Transform) (*skpath.Path, skpath.Stroke, bool) {
	st := s.ToSkia()
	st.Width *= skpath.ComputeResolutionScale(parent)
	if st.Dash == nil {
		return geometry, st, true
	}
	dash := st.Dash
	st.Dash = nil
	dashed, ok := geometry.DashPath(dash, resolutionScale)
	return dashed, st, ok
}

// isBoundingBoxPath reports a four-corner rectangle covering the whole
// 16x16 canvas (the invisible sizing rect icon sets carry).
func isBoundingBoxPath(p *skpath.Path, size float32) bool {
	it := p.Segments()
	var segs []skpath.Segment
	for {
		s, ok := it.Next()
		if !ok {
			break
		}
		segs = append(segs, s)
	}
	if len(segs) != 5 {
		return false
	}
	const eps float32 = 0.5
	abs := func(f float32) float32 {
		if f < 0 {
			return -f
		}
		return f
	}
	var corners []skpath.Point
	for _, s := range segs {
		if s.Kind == skpath.SegMove || s.Kind == skpath.SegLine {
			corners = append(corners, s.P[0])
		}
	}
	if len(corners) != 4 {
		return false
	}
	var origin, topRight, bottomRight, bottomLeft bool
	for _, c := range corners {
		left := abs(c.X) < eps
		right := abs(c.X-size) < eps
		top := abs(c.Y) < eps
		bottom := abs(c.Y-size) < eps
		switch {
		case left && top:
			origin = true
		case right && top:
			topRight = true
		case right && bottom:
			bottomRight = true
		case left && bottom:
			bottomLeft = true
		}
	}
	return origin && topRight && bottomRight && bottomLeft
}

func pathData(p *skpath.Path) string {
	var b strings.Builder
	it := p.Segments()
	for {
		s, ok := it.Next()
		if !ok {
			break
		}
		switch s.Kind {
		case skpath.SegMove:
			writeCommand(&b, 'M', s.P[0].X, s.P[0].Y)
		case skpath.SegLine:
			writeCommand(&b, 'L', s.P[0].X, s.P[0].Y)
		case skpath.SegQuad:
			writeCommand(&b, 'Q', s.P[0].X, s.P[0].Y, s.P[1].X, s.P[1].Y)
		case skpath.SegCubic:
			writeCommand(&b, 'C', s.P[0].X, s.P[0].Y, s.P[1].X, s.P[1].Y, s.P[2].X, s.P[2].Y)
		case skpath.SegClose:
			b.WriteByte('Z')
		}
	}
	return b.String()
}

func writeCommand(b *strings.Builder, cmd byte, coords ...float32) {
	b.WriteByte(cmd)
	for i, c := range coords {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatCoord(c))
	}
}

// formatCoord prints an f32 at two decimals: the exact binary value
// rounded half-to-even, "-0.00" for small negatives, "NaN"/"inf" for
// the non-finite values.
func formatCoord(f float32) string {
	switch {
	case f != f:
		return "NaN"
	case f > 3.4e38:
		return "inf"
	case f < -3.4e38:
		return "-inf"
	}
	return fmt.Sprintf("%.2f", float64(f))
}

func buildGTKSVG(paths symbolicPaths) string {
	var b strings.Builder
	b.WriteString("<svg width='16' height='16'\n")
	b.WriteString("     xmlns:gpa='https://www.gtk.org/grappa'\n")
	fmt.Fprintf(&b, "     gpa:version='%d'>\n", FormatVersion)
	if paths.stroked() {
		writeStrokeSource(&b, paths.source)
	}
	for _, p := range paths.filled {
		if !p.isBoundingBox {
			b.WriteString(buildFillPathElement(p.d))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func writeStrokeSource(b *strings.Builder, source []sourcePath) {
	b.WriteString("  <metadata>\n")
	b.WriteString("    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 16 16'>\n")
	for _, p := range source {
		if p.stroke == nil {
			b.WriteString("      <path d='" + p.d + "' fill='rgb(0,0,0)'/>\n")
			continue
		}
		b.WriteString("      <path d='" + p.d + "' fill='none' stroke='rgb(0,0,0)'")
		writeStrokeAttrs(b, p.stroke)
		b.WriteString("/>\n")
	}
	b.WriteString("    </svg>\n")
	b.WriteString("  </metadata>\n")
}

var (
	svgLineCaps  = [...]string{skpath.CapButt: "butt", skpath.CapRound: "round", skpath.CapSquare: "square"}
	svgLineJoins = [...]string{skpath.JoinMiter: "miter", skpath.JoinMiterClip: "miter-clip", skpath.JoinRound: "round", skpath.JoinBevel: "bevel"}
)

func writeStrokeAttrs(b *strings.Builder, s *skpath.Stroke) {
	b.WriteString(" stroke-width='" + formatCoord(s.Width) + "'")
	b.WriteString(" stroke-linecap='" + svgLineCaps[s.LineCap] + "'")
	b.WriteString(" stroke-linejoin='" + svgLineJoins[s.LineJoin] + "'")
	b.WriteString(" stroke-miterlimit='" + formatCoord(s.MiterLimit) + "'")
}

func buildFillPathElement(d string) string {
	return "  <path d='" + d + "'\nstroke='none'\nfill='rgb(0,0,0)'\ngpa:fill='foreground'/>\n"
}

func buildFallbackSVG(original string) (string, bool) {
	d, ok := extractPathDFallback(original)
	if !ok {
		return "", false
	}
	return buildGTKSVG(symbolicPaths{filled: []scaledPath{{d: d}}}), true
}

func extractPathDFallback(content string) (string, bool) {
	if d, ok := extractQuotedAttr(content, `d="`, '"'); ok {
		return d, true
	}
	return extractQuotedAttr(content, "d='", '\'')
}

func extractQuotedAttr(content, prefix string, quote byte) (string, bool) {
	start := strings.Index(content, prefix)
	if start < 0 {
		return "", false
	}
	start += len(prefix)
	end := strings.IndexByte(content[start:], quote)
	if end < 0 {
		return "", false
	}
	return content[start : start+end], true
}
