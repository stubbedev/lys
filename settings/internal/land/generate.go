package land

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"slices"
)

const (
	sourceVersion   = "5.1.2"
	sourceURL       = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/f1890d9f152c896d250a77557a5751a93d494776/geojson/ne_110m_land.geojson"
	sourceSHA256    = "9e0729ee253ca7d7a5c4ae9395fb1902264c5377c52e224d13dd85010e2835d9"
	toleranceDegree = 0.1
)

type geometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type featureCollection struct {
	Type     string `json:"type"`
	Features []struct {
		Geometry geometry `json:"geometry"`
	} `json:"features"`
}

type degrees struct{ longitude, latitude float64 }

func Generate(src []byte) ([]byte, error) {
	if sum := sha256.Sum256(src); hex.EncodeToString(sum[:]) != sourceSHA256 {
		return nil, fmt.Errorf("the source's sha256 is %x, not that of Natural Earth %s's ne_110m_land.geojson (%s), %s", sum, sourceVersion, sourceURL, sourceSHA256)
	}
	rings, err := landRings(src)
	if err != nil {
		return nil, fmt.Errorf("read Natural Earth %s's land: %w", sourceVersion, err)
	}
	return encode(rings), nil
}

func landRings(src []byte) ([]ring, error) {
	var fc featureCollection
	if err := json.Unmarshal(src, &fc); err != nil {
		return nil, fmt.Errorf("parse the GeoJSON: %w", err)
	}
	if fc.Type != "FeatureCollection" {
		return nil, fmt.Errorf("the GeoJSON is a %q, not a FeatureCollection", fc.Type)
	}
	var out []ring
	for i, f := range fc.Features {
		polygons, err := polygonsOf(f.Geometry)
		if err != nil {
			return nil, fmt.Errorf("feature %d: %w", i, err)
		}
		for j, polygon := range polygons {
			for k, coords := range polygon {
				r, err := landRing(coords, k == 0)
				if err != nil {
					return nil, fmt.Errorf("feature %d, polygon %d, ring %d: %w", i, j, k, err)
				}
				if r != nil {
					out = append(out, r)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the %d features hold no land", len(fc.Features))
	}
	return out, nil
}

func polygonsOf(g geometry) ([][][][]float64, error) {
	switch g.Type {
	case "Polygon":
		var polygon [][][]float64
		if err := json.Unmarshal(g.Coordinates, &polygon); err != nil {
			return nil, fmt.Errorf("parse the Polygon's coordinates: %w", err)
		}
		return [][][][]float64{polygon}, nil
	case "MultiPolygon":
		var polygons [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &polygons); err != nil {
			return nil, fmt.Errorf("parse the MultiPolygon's coordinates: %w", err)
		}
		return polygons, nil
	default:
		return nil, fmt.Errorf("the geometry is a %q, not a Polygon or MultiPolygon", g.Type)
	}
}

func landRing(coords [][]float64, exterior bool) (ring, error) {
	if len(coords) < minRingPoints+1 {
		return nil, fmt.Errorf("%d positions cannot close a ring", len(coords))
	}
	pts := make([]degrees, len(coords))
	for i, c := range coords {
		if len(c) < 2 {
			return nil, fmt.Errorf("position %d has %d coordinates", i, len(c))
		}
		if math.Abs(c[0]) > degreesWide/2 || math.Abs(c[1]) > degreesHigh/2 {
			return nil, fmt.Errorf("position %d, %g°E %g°N, is off the globe", i, c[0], c[1])
		}
		pts[i] = degrees{longitude: c[0], latitude: c[1]}
	}
	if pts[0] != pts[len(pts)-1] {
		return nil, fmt.Errorf("the ring ends at %v, not at its start %v", pts[len(pts)-1], pts[0])
	}
	simple := simplify(pts, toleranceDegree)
	r := quantise(simple[:len(simple)-1])
	area := twiceArea(r)
	if len(r) < minRingPoints || area == 0 {
		return nil, nil
	}
	if (area > 0) != exterior {
		slices.Reverse(r)
	}
	return r, nil
}

func simplify(pts []degrees, tolerance float64) []degrees {
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	spans := [][2]int{{0, len(pts) - 1}}
	for len(spans) > 0 {
		span := spans[len(spans)-1]
		spans = spans[:len(spans)-1]
		far, farthest := -1, tolerance
		for i := span[0] + 1; i < span[1]; i++ {
			if d := segmentDistance(pts[i], pts[span[0]], pts[span[1]]); d > farthest {
				far, farthest = i, d
			}
		}
		if far < 0 {
			continue
		}
		keep[far] = true
		spans = append(spans, [2]int{span[0], far}, [2]int{far, span[1]})
	}
	out := make([]degrees, 0, len(pts))
	for i, p := range pts {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

func segmentDistance(p, a, b degrees) float64 {
	dx, dy := b.longitude-a.longitude, b.latitude-a.latitude
	t := 0.0
	if span := dx*dx + dy*dy; span > 0 {
		t = math.Max(0, math.Min(1, ((p.longitude-a.longitude)*dx+(p.latitude-a.latitude)*dy)/span))
	}
	return math.Hypot(p.longitude-a.longitude-t*dx, p.latitude-a.latitude-t*dy)
}

func quantise(pts []degrees) ring {
	out := make(ring, 0, len(pts))
	for _, p := range pts {
		s := step{
			x: int64(math.Round((p.longitude + degreesWide/2) * stepsPerDegree)),
			y: int64(math.Round((p.latitude + degreesHigh/2) * stepsPerDegree)),
		}
		if len(out) > 0 && out[len(out)-1] == s {
			continue
		}
		out = append(out, s)
	}
	for len(out) > 1 && out[len(out)-1] == out[0] {
		out = out[:len(out)-1]
	}
	return out
}

func twiceArea(r ring) int64 {
	var sum int64
	for i, p := range r {
		q := r[(i+1)%len(r)]
		sum += p.x*q.y - q.x*p.y
	}
	return sum
}
