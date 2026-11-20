package config

// ThresholdsConfig is the warning levels of the machine's resources:
// one set per resource, which its bar module and the dashboard both
// show.
//
// Warning levels of CPU, RAM, disk and battery, shared by their bar
// modules and the dashboard.
//
// Each resource is a list of thresholds over its percentage. Entries are
// checked in order; the last matching entry wins for each color slot.
// The bar module recolors its button with the slots; the dashboard's
// gauge for the resource takes the first color an entry sets, in the
// order icon, label, button background, icon background, border, and
// its battery card colors its icon with `icon-color` and its text with
// `label-color`.
//
// ## Example
//
// ```yaml
// thresholds: {cpu: [{above: 70, icon-color: status-warning}, {above: 90, icon-color: status-error}]}
// ```
type ThresholdsConfig struct {
	// CPU usage percentage.
	CPU []ThresholdEntry `cfg:"cpu"`
	// RAM usage percentage.
	RAM []ThresholdEntry `cfg:"ram"`
	// Disk usage percentage of the storage module's mount, and of the
	// root file system on the dashboard.
	Storage []ThresholdEntry `cfg:"storage"`
	// Battery charge percentage. Use `below` for low-charge warnings.
	Battery []ThresholdEntry `cfg:"battery"`
}

// DefaultsThresholds returns the schema defaults: warning, then error,
// past 60 % and 85 % of use and under 30 % and 15 % of charge.
func DefaultsThresholds() ThresholdsConfig {
	usage := func() []ThresholdEntry {
		return []ThresholdEntry{
			statusThreshold(new(60.0), nil, TokenStatusWarning),
			statusThreshold(new(85.0), nil, TokenStatusError),
		}
	}
	return ThresholdsConfig{
		CPU:     usage(),
		RAM:     usage(),
		Storage: usage(),
		Battery: []ThresholdEntry{
			statusThreshold(nil, new(30.0), TokenStatusWarning),
			statusThreshold(nil, new(15.0), TokenStatusError),
		},
	}
}

func statusThreshold(above, below *float64, token CssToken) ThresholdEntry {
	icon := ColorValue{Kind: ColorToken, Token: token}
	label := icon
	return ThresholdEntry{Above: above, Below: below, IconColor: &icon, LabelColor: &label}
}

// ThresholdEntry maps a numeric range to color overrides.
//
// A threshold entry that maps a numeric value range to color overrides.
//
// At least one of `above` or `below` must be set. When both are set,
// both conditions must be satisfied (AND logic).
//
// ## YAML Example
//
// ```yaml
// # under thresholds
// cpu:
// - {above: 70, icon-color: status-warning, label-color: status-warning}
// - {above: 90, icon-color: status-error, label-color: status-error}
// ```
type ThresholdEntry struct {
	// Activate when metric value >= this threshold.
	Above *float64 `cfg:"above"`
	// Activate when metric value <= this threshold.
	Below *float64 `cfg:"below"`
	// Override icon color when threshold is active.
	IconColor *ColorValue `cfg:"icon-color"`
	// Override label color when threshold is active.
	LabelColor *ColorValue `cfg:"label-color"`
	// Override icon background color when threshold is active.
	IconBgColor *ColorValue `cfg:"icon-bg-color"`
	// Override button background color when threshold is active.
	ButtonBgColor *ColorValue `cfg:"button-bg-color"`
	// Override border color when threshold is active.
	BorderColor *ColorValue `cfg:"border-color"`
}

// Matches reports whether value falls in the entry's range: both set
// bounds hold (inclusive), and an entry with neither bound never
// matches.
func (t ThresholdEntry) Matches(value float64) bool {
	if t.Above == nil && t.Below == nil {
		return false
	}
	if t.Above != nil && value < *t.Above {
		return false
	}
	if t.Below != nil && value > *t.Below {
		return false
	}
	return true
}

// ThresholdColors is the merged override set of every matching entry;
// a nil slot keeps the module's configured color.
type ThresholdColors struct {
	IconColor     *ColorValue
	LabelColor    *ColorValue
	IconBgColor   *ColorValue
	ButtonBgColor *ColorValue
	BorderColor   *ColorValue
}

// IsEmpty reports whether no slot is overridden.
func (c ThresholdColors) IsEmpty() bool {
	return c.IconColor == nil && c.LabelColor == nil && c.IconBgColor == nil &&
		c.ButtonBgColor == nil && c.BorderColor == nil
}

// Color is the one color a gauge shows the matching entries in: the
// first slot set, in the order icon, label, button background, icon
// background, border; nil when none is.
func (c ThresholdColors) Color() *ColorValue {
	for _, slot := range []*ColorValue{c.IconColor, c.LabelColor, c.ButtonBgColor, c.IconBgColor, c.BorderColor} {
		if slot != nil {
			return slot
		}
	}
	return nil
}

// ResolveOr is one slot's CSS: the override's value when set, else the
// module's configured color already resolved for the theme.
func ResolveOr(override *ColorValue, configCSS string) string {
	if override != nil {
		return override.ToCSS()
	}
	return configCSS
}

// EvaluateThresholds walks the entries in order; for each color slot
// the last matching entry that sets it wins (evaluate_thresholds). The
// slots are copies, so a caller cannot alias the config.
func EvaluateThresholds(value float64, entries []ThresholdEntry) ThresholdColors {
	var out ThresholdColors
	for _, e := range entries {
		if !e.Matches(value) {
			continue
		}
		for _, c := range []struct {
			src *ColorValue
			dst **ColorValue
		}{
			{e.IconColor, &out.IconColor},
			{e.LabelColor, &out.LabelColor},
			{e.IconBgColor, &out.IconBgColor},
			{e.ButtonBgColor, &out.ButtonBgColor},
			{e.BorderColor, &out.BorderColor},
		} {
			if c.src != nil {
				cv := *c.src
				*c.dst = &cv
			}
		}
	}
	return out
}
