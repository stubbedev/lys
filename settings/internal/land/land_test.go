package land

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTheCommittedAssetIsTheGeneratorsOutputOfTheSource(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("source", "ne_110m_land.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Generate(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(asset, want) {
		t.Fatal("settings/internal/land/land.bin is stale: run go generate ./settings/internal/land")
	}
	if len(asset) > 10_000 {
		t.Errorf("the asset is %d bytes, more than the 10 kB a 1:110m land map needs", len(asset))
	}
}

func TestASourceOtherThanThePinnedOneIsRefused(t *testing.T) {
	_, err := Generate([]byte(`{"type":"FeatureCollection","features":[]}`))
	if err == nil || !strings.Contains(err.Error(), sourceSHA256) || !strings.Contains(err.Error(), sourceVersion) {
		t.Errorf("an unpinned source: %v", err)
	}
}

func TestTheAssetDecodesToRingsOnTheGlobe(t *testing.T) {
	rings := decoded()
	if len(rings) < 100 {
		t.Fatalf("%d rings, fewer than Natural Earth's land holds", len(rings))
	}
	holes := 0
	for _, r := range rings {
		if twiceArea(r) < 0 {
			holes++
		}
	}
	if holes != 1 {
		t.Errorf("%d rings wind as holes, want the Caspian Sea alone", holes)
	}
}

func TestRingsAreSimplifiedQuantisedAndWoundExteriorOutHoleIn(t *testing.T) {
	src := `{"type":"FeatureCollection","features":[
		{"geometry":{"type":"Polygon","coordinates":[
			[[0,0],[0,4],[2,4.05],[4,4],[4,0],[2,0.0001],[0,0]],
			[[1,1],[3,1],[3,3],[1,3],[1,1]]
		]}},
		{"geometry":{"type":"MultiPolygon","coordinates":[[
			[[10,10],[10.01,10],[10.01,10.01],[10,10]]
		]]}}
	]}`
	rings, err := landRings([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := []ring{
		{{3680, 1800}, {3680, 1880}, {3600, 1880}, {3600, 1800}},
		{{3620, 1860}, {3660, 1860}, {3660, 1820}, {3620, 1820}},
	}
	if !slices.EqualFunc(rings, want, slices.Equal) {
		t.Errorf("rings %v, want %v", rings, want)
	}
	again, err := decode(encode(rings))
	if err != nil || !slices.EqualFunc(again, want, slices.Equal) {
		t.Errorf("round trip %v, %v", again, err)
	}
}

func TestAMalformedSourceIsRefused(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{`[]`, "parse the GeoJSON"},
		{`{"type":"Feature"}`, `a "Feature", not a FeatureCollection`},
		{`{"type":"FeatureCollection","features":[]}`, "hold no land"},
		{`{"type":"FeatureCollection","features":[{"geometry":{"type":"Point","coordinates":[0,0]}}]}`, `feature 0: the geometry is a "Point"`},
		{`{"type":"FeatureCollection","features":[{"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}}]}`, "3 positions cannot close a ring"},
		{`{"type":"FeatureCollection","features":[{"geometry":{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}}]}`, "not at its start"},
		{`{"type":"FeatureCollection","features":[{"geometry":{"type":"Polygon","coordinates":[[[0,0],[181,0],[1,1],[0,0]]]}}]}`, "position 1, 181°E 0°N, is off the globe"},
		{`{"type":"FeatureCollection","features":[{"geometry":{"type":"Polygon","coordinates":[[[0,0],[1],[1,1],[0,0]]]}}]}`, "position 1 has 1 coordinates"},
	} {
		if _, err := landRings([]byte(tc.src)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.src, err, tc.want)
		}
	}
}

func TestACorruptAssetIsRefused(t *testing.T) {
	good := encode([]ring{{{0, 0}, {10, 0}, {10, 10}}})
	for _, tc := range []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "read the ring count"},
		{"truncated", good[:len(good)-1], "read ring 0's point 2"},
		{"trailing", append(slices.Clone(good), 0), "1 bytes follow the last ring"},
		{"too few points", encode([]ring{{{0, 0}, {1, 1}}}), "ring 0 has 2 points"},
		{"off the globe", encode([]ring{{{0, 0}, {stepsWide + 1, 0}, {0, 1}}}), "ring 0's point 1"},
		{"too many rings", []byte{0x7f}, "127 rings cannot fit in 1 bytes"},
	} {
		_, err := decode(tc.data)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	if _, err := decode(good[:len(good)-1]); !errors.Is(err, errTruncated) {
		t.Errorf("a truncated asset: %v, want errTruncated", err)
	}
}
