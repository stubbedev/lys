package drm_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/drm"
)

const orientationID = 77

type infos map[uint32]drm.PropertyInfo

func (i infos) PropertyInfo(id uint32) (drm.PropertyInfo, error) {
	info, ok := i[id]
	if !ok {
		return drm.PropertyInfo{}, errors.New("no such property")
	}
	return info, nil
}

var kernelOrientations = infos{orientationID: {Name: "panel orientation", Enums: map[string]uint64{"Normal": 0, "Upside Down": 1, "Left Side Up": 2, "Right Side Up": 3}}}

func orientationProps(value uint64) map[string]drm.Property {
	return map[string]drm.Property{"panel orientation": {ID: orientationID, Value: value}}
}

func TestPanelOrientationOfReadsTheEnumerator(t *testing.T) {
	t.Parallel()
	for value, want := range []drm.PanelOrientation{drm.PanelNormal, drm.PanelUpsideDown, drm.PanelLeftSideUp, drm.PanelRightSideUp} {
		got, err := drm.PanelOrientationOf(orientationProps(uint64(value)), kernelOrientations)
		if err != nil || got != want {
			t.Errorf("value %d: %v, %v; want %v", value, got, err, want)
		}
	}
}

func TestAConnectorWithoutAPanelOrientationIsUpright(t *testing.T) {
	t.Parallel()
	got, err := drm.PanelOrientationOf(map[string]drm.Property{"CRTC_ID": {ID: 1}}, infos{})
	if err != nil || got != drm.PanelNormal {
		t.Fatalf("%v, %v; want Normal", got, err)
	}
}

func TestPanelOrientationOfRefusesWhatItCannotName(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		props map[string]drm.Property
		infos infos
		want  string
	}{
		"a value with no enumerator": {orientationProps(9), kernelOrientations, "panel orientation 9 is none of"},
		"an unknown enumerator":      {orientationProps(4), infos{orientationID: {Name: "panel orientation", Enums: map[string]uint64{"Sideways": 4}}}, `unknown panel orientation "Sideways"`},
		"an unreadable property":     {orientationProps(0), infos{}, "read the panel orientation property 77"},
	} {
		if got, err := drm.PanelOrientationOf(c.props, c.infos); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, %v; want an error with %q", name, got, err, c.want)
		}
	}
}

func TestParsePanelOrientationRoundTrips(t *testing.T) {
	t.Parallel()
	for _, o := range []drm.PanelOrientation{drm.PanelNormal, drm.PanelUpsideDown, drm.PanelLeftSideUp, drm.PanelRightSideUp} {
		if got, err := drm.ParsePanelOrientation(o.String()); err != nil || got != o {
			t.Errorf("%v parses back as %v, %v", o, got, err)
		}
	}
	if _, err := drm.ParsePanelOrientation("Sideways"); err == nil {
		t.Error("an unknown orientation parsed")
	}
}
