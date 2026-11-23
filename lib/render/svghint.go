package render

import (
	"math"
	"slices"

	"golang.org/x/image/math/fixed"
)

type svgFill struct {
	points  []svgPoint
	evenOdd bool
}

type svgGrid struct {
	x, y axisHint
}

func (g *svgGrid) place(p fixed.Point26_6) fixed.Point26_6 {
	return fixed.Point26_6{
		X: fixed.Int26_6(math.Round(g.x.device(float64(p.X)/64) * 64)),
		Y: fixed.Int26_6(math.Round(g.y.device(float64(p.Y)/64) * 64)),
	}
}

func fitSVGGrid(fills []svgFill, size float64) svgGrid {
	flat := float64(size * edgeFlatnessPerEm)
	minLength := float64(size * edgeMinLengthPerEm)
	var columns, rows []hintEdge
	for _, f := range fills {
		forEachSegment(f.points, func(a, b svgPoint) {
			ax, ay, bx, by := float64(a.x), float64(a.y), float64(b.x), float64(b.y)
			mx, my := float64((ax+bx)/2), float64((ay+by)/2)
			if math.Abs(by-ay) <= flat && math.Abs(bx-ax) >= minLength {
				if before, after := inked(fills, mx, my-flat), inked(fills, mx, my+flat); before != after {
					rows = append(rows, hintEdge{at: my, lo: math.Min(ax, bx), hi: math.Max(ax, bx), inkAfter: after})
				}
			}
			if math.Abs(bx-ax) <= flat && math.Abs(by-ay) >= minLength {
				if before, after := inked(fills, mx-flat, my), inked(fills, mx+flat, my); before != after {
					columns = append(columns, hintEdge{at: mx, lo: math.Min(ay, by), hi: math.Max(ay, by), inkAfter: after})
				}
			}
		})
	}
	maxStem := float64(size * stemMaxWidthPerEm)
	return svgGrid{x: fitAxis(columns, maxStem), y: fitAxis(rows, maxStem)}
}

func fitAxis(edges []hintEdge, maxStem float64) axisHint {
	h := axisHint{scale: 1, anchors: make([]gridAnchor, 0, len(edges))}
	stemmed := make([]bool, len(edges))
	for _, s := range pairStems(edges, maxStem) {
		lo, hi := edges[s.lo].at, edges[s.hi].at
		width := math.Max(1, math.Ceil(hi-lo-0.5))
		start := math.Round(float64((lo+hi)/2) - float64(width/2))
		h = h.with(lo, start).with(hi, start+width)
		stemmed[s.lo], stemmed[s.hi] = true, true
	}
	for i, e := range edges {
		if !stemmed[i] {
			h = h.with(e.at, math.Round(e.at))
		}
	}
	return h
}

func forEachSegment(points []svgPoint, visit func(a, b svgPoint)) {
	for i := 0; i < len(points); {
		j := i + 1
		for j < len(points) && !points[j].start {
			j++
		}
		sub := points[i:j]
		for k := range sub {
			visit(sub[k], sub[(k+1)%len(sub)])
		}
		i = j
	}
}

func inked(fills []svgFill, x, y float64) bool {
	return slices.ContainsFunc(fills, func(f svgFill) bool {
		w := winding(f.points, x, y)
		if f.evenOdd {
			return w%2 != 0
		}
		return w != 0
	})
}

func winding(points []svgPoint, x, y float64) int {
	w := 0
	forEachSegment(points, func(a, b svgPoint) {
		ax, ay, bx, by := float64(a.x), float64(a.y), float64(b.x), float64(b.y)
		side := float64((bx-ax)*(y-ay)) - float64((x-ax)*(by-ay))
		switch {
		case ay <= y && by > y && side > 0:
			w++
		case by <= y && ay > y && side < 0:
			w--
		}
	})
	return w
}
