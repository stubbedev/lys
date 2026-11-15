package bar

import (
	"errors"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
	"github.com/stubbedev/lys/shell/service/weather"
)

// weatherConditionIDs is the condition vocabulary: one
// _weather.ftl message per condition.
var weatherConditionIDs = map[weather.Condition]string{
	weather.CondClear:        "weather-clear",
	weather.CondPartlyCloudy: "weather-partly-cloudy",
	weather.CondCloudy:       "weather-cloudy",
	weather.CondOvercast:     "weather-overcast",
	weather.CondMist:         "weather-mist",
	weather.CondFog:          "weather-fog",
	weather.CondLightRain:    "weather-light-rain",
	weather.CondRain:         "weather-rain",
	weather.CondHeavyRain:    "weather-heavy-rain",
	weather.CondDrizzle:      "weather-drizzle",
	weather.CondLightSnow:    "weather-light-snow",
	weather.CondSnow:         "weather-snow",
	weather.CondHeavySnow:    "weather-heavy-snow",
	weather.CondSleet:        "weather-sleet",
	weather.CondThunderstorm: "weather-thunderstorm",
	weather.CondWindy:        "weather-windy",
	weather.CondHail:         "weather-hail",
	weather.CondUnknown:      "weather-unknown",
}

func weatherConditionLabel(c weather.Condition) string {
	id, ok := weatherConditionIDs[c]
	if !ok {
		id = weatherConditionIDs[weather.CondUnknown]
	}
	return i18n.T(id)
}

// weatherConditionIcon picks the condition glyph: day and night
// variants where the glyph set has them.
func weatherConditionIcon(c weather.Condition, isDay bool) string {
	switch c {
	case weather.CondClear:
		if isDay {
			return "ld-sun-symbolic"
		}
		return "ld-moon-symbolic"
	case weather.CondPartlyCloudy:
		if isDay {
			return "ld-cloud-sun-symbolic"
		}
		return "ld-cloud-moon-symbolic"
	case weather.CondCloudy:
		return "ld-cloudy-symbolic"
	case weather.CondMist:
		return "ld-haze-symbolic"
	case weather.CondFog:
		return "ld-cloud-fog-symbolic"
	case weather.CondLightRain:
		if isDay {
			return "ld-cloud-sun-rain-symbolic"
		}
		return "ld-cloud-moon-rain-symbolic"
	case weather.CondRain:
		return "ld-cloud-rain-symbolic"
	case weather.CondHeavyRain:
		return "ld-cloud-rain-wind-symbolic"
	case weather.CondDrizzle:
		return "ld-cloud-drizzle-symbolic"
	case weather.CondLightSnow, weather.CondSnow, weather.CondHeavySnow:
		return "ld-cloud-snow-symbolic"
	case weather.CondSleet, weather.CondHail:
		return "ld-cloud-hail-symbolic"
	case weather.CondThunderstorm:
		return "ld-cloud-lightning-symbolic"
	case weather.CondWindy:
		return "ld-wind-symbolic"
	}
	return "ld-cloud-symbolic"
}

// weatherConditionClass is the dropdown's
// condition tint.
func weatherConditionClass(c weather.Condition) string {
	switch c {
	case weather.CondClear, weather.CondPartlyCloudy:
		return "sunny"
	case weather.CondLightRain, weather.CondRain, weather.CondHeavyRain, weather.CondDrizzle:
		return "rainy"
	case weather.CondThunderstorm, weather.CondHail:
		return "stormy"
	case weather.CondLightSnow, weather.CondSnow, weather.CondHeavySnow, weather.CondSleet:
		return "snowy"
	}
	return "cloudy"
}

// weatherTemp: whole degrees in the unit system
// ({:.0} of the f32).
func weatherTemp(t weather.Temperature, imperial bool) string {
	v := t.Celsius()
	if imperial {
		v = t.Fahrenheit()
	}
	return strconv.FormatFloat(float64(v), 'f', 0, 32)
}

func weatherUnitSymbol(imperial bool) string {
	if imperial {
		return "°F"
	}
	return "°C"
}

func weatherSpeed(s weather.Speed, imperial bool) string {
	if imperial {
		return strconv.FormatFloat(float64(s.Mph()), 'f', 0, 32) + " mph"
	}
	return strconv.FormatFloat(float64(s.Kmh()), 'f', 0, 32) + " km/h"
}

// weatherFormatLabel renders format over the schema's
// nine placeholders; today's high and low render empty without a
// daily forecast.
func weatherFormatLabel(format string, w *weather.Weather, imperial bool) string {
	return jinja.RenderOr(format, weatherVars(w, imperial))
}

func weatherVars(w *weather.Weather, imperial bool) map[string]any {
	c := w.Current
	high, low := "", ""
	if len(w.Daily) > 0 {
		high, low = weatherTemp(w.Daily[0].TempHigh, imperial), weatherTemp(w.Daily[0].TempLow, imperial)
	}
	return map[string]any{
		"temp":       weatherTemp(c.Temperature, imperial),
		"temp_unit":  weatherUnitSymbol(imperial),
		"feels_like": weatherTemp(c.FeelsLike, imperial),
		"condition":  weatherConditionLabel(c.Condition),
		"humidity":   strconv.Itoa(int(c.Humidity)) + "%",
		"wind_speed": weatherSpeed(c.WindSpeed, imperial),
		"wind_dir":   c.WindDirection.Cardinal(),
		"high":       high,
		"low":        low,
	}
}

// weatherSettings maps [modules.weather] onto the service, resolving
// the API keys ($NAME reads the environment).
func weatherSettings(cfg config.WeatherConfig) weather.Settings {
	s := weather.Settings{
		Interval: time.Duration(cfg.RefreshIntervalSeconds) * time.Second,
		Location: weather.ParseLocation(cfg.Location),
	}
	switch cfg.Provider {
	case config.WeatherProviderVisualCrossing:
		s.Provider = weather.VisualCrossing
	case config.WeatherProviderWeatherApi:
		s.Provider = weather.WeatherAPI
	default:
		s.Provider = weather.OpenMeteo
	}
	s.VisualCrossingKey = resolvedSecret(cfg.VisualCrossingKey)
	s.WeatherAPIKey = resolvedSecret(cfg.WeatherapiKey)
	return s
}

func resolvedSecret(value *string) string {
	if value == nil {
		return ""
	}
	key, _ := config.ResolveSecret(*value)
	return key
}

// weatherModule is the bar button: the formatted conditions and their
// icon, re-rendered on every service change.
type weatherModule struct {
	ctx    ModuleContext
	svc    *weather.Service
	btn    *barButton
	icon   *widget.Icon
	chrome moduleChrome
}

func newWeather(ctx ModuleContext) (Module, error) {
	if ctx.Weather == nil {
		return nil, errors.New("weather: no weather service")
	}
	m := &weatherModule{ctx: ctx, svc: ctx.Weather}
	m.icon = moduleIcon(ctx, ctx.Config.Weather.Icon())
	m.btn = newBarButton(ctx, m.icon, "--")
	changes, stop := m.svc.Subscribe()
	m.render()
	follow(ctx, changes, stop, func(struct{}) { m.render() })
	return m, nil
}

// render is the weather watcher: without data the configured icon
// stays and the label keeps its last text; with data the label is the
// format and the icon the condition's. A failed refresh over held data
// marks the module weather-stale and says why in its tooltip.
func (m *weatherModule) render() {
	cfg := m.ctx.Config.Weather
	w := m.svc.Weather()
	st := m.svc.Status()
	stale := weatherStale(st, w)
	setClass(m.chrome, weatherStaleClass, stale)
	if w == nil {
		m.setIcon(cfg.IconName)
		m.btn.SetDescription(weatherFailure(st))
		return
	}
	label := weatherFormatLabel(cfg.Format, w, cfg.Units == config.TemperatureUnitImperial)
	m.btn.SetLabel(label)
	m.setIcon(weatherConditionIcon(w.Current.Condition, w.Current.IsDay))
	detail := ""
	if stale {
		detail = i18n.T("weather-stale", i18n.Str("reading", label), i18n.Str("reason", weatherErrorText(st)))
	}
	m.btn.SetDescription(detail)
}

const weatherStaleClass = "weather-stale"

func weatherStale(st weather.Status, w *weather.Weather) bool {
	return st.Kind == weather.Failed && w != nil
}

func weatherFailure(st weather.Status) string {
	if st.Kind != weather.Failed {
		return ""
	}
	return weatherErrorText(st)
}

func (m *weatherModule) setChrome(c moduleChrome) {
	m.chrome = c
	m.render()
}

func (m *weatherModule) setIcon(name string) {
	if m.icon != nil {
		m.icon.SetThemeName(name)
	}
}

func (m *weatherModule) Root() widget.Widget { return m.btn }
