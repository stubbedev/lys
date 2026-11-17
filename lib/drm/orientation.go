package drm

import "fmt"

// PanelOrientation is how a panel is mounted, read from its connector's
// "panel orientation" property: the side of the panel that is up.
type PanelOrientation uint8

// The orientations the kernel defines for "panel orientation".
const (
	PanelNormal PanelOrientation = iota
	PanelUpsideDown
	PanelLeftSideUp
	PanelRightSideUp
)

const propPanelOrientation = "panel orientation"

var panelOrientations = [...]string{PanelNormal: "Normal", PanelUpsideDown: "Upside Down", PanelLeftSideUp: "Left Side Up", PanelRightSideUp: "Right Side Up"}

// ParsePanelOrientation maps a "panel orientation" enumerator name to a
// PanelOrientation.
func ParsePanelOrientation(name string) (PanelOrientation, error) {
	for o, n := range panelOrientations {
		if n == name {
			return PanelOrientation(o), nil
		}
	}
	return PanelNormal, fmt.Errorf("drm: unknown panel orientation %q", name)
}

func (o PanelOrientation) String() string {
	if int(o) < len(panelOrientations) {
		return panelOrientations[o]
	}
	return fmt.Sprintf("PanelOrientation(%d)", uint8(o))
}

// PropertyInfos resolves a property ID to its name and enumerators, as
// Device.PropertyInfo does.
type PropertyInfos interface {
	PropertyInfo(id uint32) (PropertyInfo, error)
}

// PanelOrientationOf reads the orientation from a connector's
// properties, resolving the enumerator through infos. A connector without
// the property is a panel mounted upright.
func PanelOrientationOf(props map[string]Property, infos PropertyInfos) (PanelOrientation, error) {
	p, ok := props[propPanelOrientation]
	if !ok {
		return PanelNormal, nil
	}
	info, err := infos.PropertyInfo(p.ID)
	if err != nil {
		return PanelNormal, fmt.Errorf("drm: read the %s property %d: %w", propPanelOrientation, p.ID, err)
	}
	for name, v := range info.Enums {
		if v == p.Value {
			return ParsePanelOrientation(name)
		}
	}
	return PanelNormal, fmt.Errorf("drm: panel orientation %d is none of %v", p.Value, info.Enums)
}
