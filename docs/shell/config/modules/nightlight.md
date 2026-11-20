---
title: nightlight
outline: [2, 3]
---

# nightlight

<div v-pre>

Night light: warms the display colors, by hand or on a schedule.

Add it to your layout with `nightlight`:

```yaml
bar:
  layout:
    - monitor: "*"
      right: [nightlight]
```

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `format` | string | `"{{ status }}"` | Format string for the label. |
| `temperature` | u32 | `5000` | Color temperature in Kelvin when filter is enabled. Range: 1000-20000. |
| `gamma` | u32 | `100` | Display gamma percentage when filter is enabled. Range: 0-200. |
| `auto-schedule` | bool | `false` | Automatically ease the filter on and off on `schedule`: at dusk and dawn, following the sun at the machine's location, or at the custom hours. |
| `schedule` | [`NightlightSchedule`](/shell/config/types#nightlight-schedule) | `"sunset-to-sunrise"` | What the schedule follows: the sun, from sunset to sunrise, or the custom hours. Only used when `auto-schedule` is on. |
| `custom-hours` | [`NightlightHours`](/shell/config/types#nightlight-hours) | `{...}` | When the custom schedule turns the filter on (`from`) and off (`to`), as HH:MM: the next day when `to` is not after `from`. Only used when `schedule` is `custom`. |
| `transition-duration` | [`NightlightTransition`](/shell/config/types#nightlight-transition) | `15` | Length of the transition between day and night, in whole minutes, centered on civil dawn or dusk, or on the custom hours. 0 switches at once. Range: 0-120. |
| `location` | [`AutoPlace`](/shell/config/types#auto-place) | `"auto"` | Where the schedule follows the sun: `"auto"` asks GeoClue, a place (`name`, `latitude`, `longitude`) pins it. Only used when `auto-schedule` is on and `schedule` is `sunset-to-sunrise`. |
| `icon-off` | string | `"ld-sun-symbolic"` | Icon when filter is disabled (showing normal daylight colors). |
| `icon-on` | string | `"ld-moon-symbolic"` | Icon when filter is enabled (showing warm night colors). |
| `border-show` | bool | `false` | Display border around button. |
| `icon-show` | bool | `true` | Display module icon. |
| `tooltip` | bool | `true` | Show the module's state in a tooltip on hover. |
| `label-show` | bool | `true` | Display label. |
| `label-max-length` | u32 | `0` | Max label characters before truncation with ellipsis. Set to 0 to disable. |

::: details More about `format`

#### Placeholders

- `{{ status }}` - Filter status (On, Off)
- `{{ temp }}` - Current temperature in Kelvin (-- while off)
- `{{ gamma }}` - Current gamma percentage (-- while off)
- `{{ config_temp }}` - Configured temperature in Kelvin
- `{{ config_gamma }}` - Configured gamma percentage

#### Examples

- `"{{ status }}"` - "On"
- `"{{ temp }}K {{ gamma }}%"` - "4500K 80%"
- `"{{ status }} ({{ temp }}K)"` - "On (4500K)"

:::

::: details More about `temperature`

The night light card in the brightness dropdown sets it, from 1000K to
6600K in steps of 100K, next to a switch that turns the filter on and
off as the module's toggle does. The card and the module show the same
night light, which runs whether or not the bar shows the module. A
strength or toggle the display refuses shows its error in the card and
the module's tooltip.

:::

::: details More about `auto-schedule`

Transitions are centered on civil dawn and dusk (sun 6 degrees below the
horizon), or on the custom hours' start and end. During polar day or
night the sun's schedule keeps the filter off or on.

The location is `location`: a pinned place, or with `"auto"` GeoClue's
fix, the last of which is kept in the state directory. Without a usable location the schedule does not run;
the tooltip and the log say why.

When no GeoClue agent runs, lys-shell registers as one during the
lookup, which needs it on GeoClue's agent whitelist. The deb and rpm
install `/etc/geoclue/conf.d/50-lys-shell.conf`, and the NixOS module's
`autoInstallDependencies` sets it up. From the tarball, install its
`geoclue-lys-shell.conf` at that path and restart geoclue. GeoClue
before 2.7 reads no `conf.d`: add the file's `lys-shell` entry to
`/etc/geoclue/geoclue.conf` and `lys-shell` to `whitelist` under
`[agent]`. A drop-in `whitelist` replaces the main file's, so the
shipped one repeats upstream's list; add any agents you rely on.

When GeoClue answers but finds no fix, it has no source that can place
the machine: set `url` under `[wifi]`, describe the location in
`/etc/geolocation`, or pin a place in `location`.

A manual click cancels a running transition, applies at once, and holds
until the next transition begins.

:::

::: details More about `location`

The former `latitude` and `longitude` keys still read, as a place
without a name; 0, 0 among them is taken as unset.

:::

## Colors

| Field | Type | Default | Description |
|---|---|---|---|
| `border-color` | [`ColorValue`](/shell/config/types#color-value) | `"yellow"` | Border color token. |
| `icon-color` | [`ColorValue`](/shell/config/types#color-value) | `"auto"` | Icon foreground color. Auto selects based on variant for contrast. |
| `icon-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"yellow"` | Icon container background color token. |
| `label-color` | [`ColorValue`](/shell/config/types#color-value) | `"fg-default"` | Label text color token. |
| `button-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"bg-surface-elevated"` | Button background color token. |

## Click actions

| Field | Type | Default | Description |
|---|---|---|---|
| `left-click` | [`ClickAction`](/shell/config/types#click-action) | `":toggle"` | Action on left click. Default toggles blue light filter. |
| `right-click` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on right click. |
| `middle-click` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on middle click. |
| `scroll-up` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on scroll up. |
| `scroll-down` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on scroll down. |

## Default configuration

```yaml
modules:
  nightlight:
    format: '{{ status }}'
    temperature: 5000
    gamma: 100
    auto-schedule: false
    schedule: sunset-to-sunrise
    custom-hours:
      from: 21:00
      to: 07:00
    transition-duration: 15
    location: auto
    icon-off: ld-sun-symbolic
    icon-on: ld-moon-symbolic
    border-show: false
    border-color: yellow
    icon-show: true
    tooltip: true
    icon-color: auto
    icon-bg-color: yellow
    label-show: true
    label-color: fg-default
    label-max-length: 0
    button-bg-color: bg-surface-elevated
    left-click: :toggle
    right-click: ""
    middle-click: ""
    scroll-up: ""
    scroll-down: ""
```


</div>
