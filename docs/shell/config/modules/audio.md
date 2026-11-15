---
title: audio
outline: [2, 3]
---

# audio

<div v-pre>

Speaker volume and mute, the microphone's state, and a capture indicator.

The button shows the default output's volume and mute state and the
default input's mute state. While an app records the microphone the
microphone icon takes the capture color; while an app holds a camera
open or shares the screen, an icon for each shows in the capture color.
Its dropdown holds the output and input devices, the app volumes, and
every app capturing, each with a button that stops it.

Add it to your layout with `audio`:

```yaml
bar:
  layout:
    - monitor: "*"
      right: [audio]
```

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `border-show` | bool | `false` | Display border around button. |
| `icon-show` | bool | `true` | Display the speaker icon. |
| `tooltip` | bool | `true` | Show the module's state in a tooltip on hover. |
| `label-show` | bool | `true` | Display the label. |
| `format` | string | `"{{ percent }}%"` | Format string for the label. |
| `label-max-length` | u32 | `0` | Max label characters before truncation with ellipsis. Set to 0 to disable. |
| `thresholds` | array of [`ThresholdEntry`](/shell/config/types#threshold-entry) | `[]` | Dynamic color thresholds based on the speaker volume percentage. |

::: details More about `format`

#### Placeholders

- `{{ percent }}` - Speaker volume (0-100)
- `{{ mic_percent }}` - Microphone volume (0-100), empty without a microphone

#### Examples

- `"{{ percent }}%"` - "45%"
- `"{{ percent }}% · {{ mic_percent }}%"` - "45% · 80%"

:::

::: details More about `thresholds`

Entries are checked in order; the last matching entry wins for each
color slot. Use `above` for high-value warnings (e.g., boosted volume).

#### Example

```yaml
modules:
  audio:
    thresholds:
      - above: 100
        icon-color: status-warning
        label-color: status-warning
      - above: 130
        icon-color: status-error
        label-color: status-error
```

:::

## Speaker

| Field | Type | Default | Description |
|---|---|---|---|
| `speaker-icons` | array of string | `[...]` | Speaker icons for volume levels from low to maximum. |
| `speaker-icon-muted` | string | `"ld-volume-x-symbolic"` | Speaker icon shown while the output is muted. |

::: details More about `speaker-icons`

The percentage is divided evenly among icons. With 3 icons:
1-33% uses speaker-icons\[0\], 34-66% uses speaker-icons\[1\],
67-100% uses speaker-icons\[2\].

:::

## Microphone

| Field | Type | Default | Description |
|---|---|---|---|
| `mic-icon` | string | `"ld-mic-symbolic"` | Microphone icon shown while the input is live (unmuted). |
| `mic-icon-muted` | string | `"ld-mic-off-symbolic"` | Microphone icon shown while the input is muted. |
| `mic-hide-when-idle` | bool | `false` | Show the microphone icon only while an app records the microphone. |

## Capture indicator

| Field | Type | Default | Description |
|---|---|---|---|
| `capture-camera-icon` | string | `"ld-webcam-symbolic"` | Icon shown while an app holds a camera open. |
| `capture-screen-icon` | string | `"ld-monitor-symbolic"` | Icon shown while an app shares the screen. |
| `capture-color` | [`ColorValue`](/shell/config/types#color-value) | `"status-warning"` | Color of the capture indicator: the microphone icon while an app records it, and the camera and screen icons. |

## Colors

| Field | Type | Default | Description |
|---|---|---|---|
| `border-color` | [`ColorValue`](/shell/config/types#color-value) | `"red"` | Border color token. |
| `icon-color` | [`ColorValue`](/shell/config/types#color-value) | `"auto"` | Icon foreground color. Auto selects based on variant for contrast. |
| `icon-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"red"` | Icon container background color token. |
| `label-color` | [`ColorValue`](/shell/config/types#color-value) | `"fg-default"` | Label text color token. |
| `button-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"bg-surface-elevated"` | Button background color token. |

## Click actions

| Field | Type | Default | Description |
|---|---|---|---|
| `left-click` | [`ClickAction`](/shell/config/types#click-action) | `"dropdown:audio"` | Action on left click. Default opens the audio dropdown. |
| `right-click` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on right click. |
| `middle-click` | [`ClickAction`](/shell/config/types#click-action) | `"lys-shell audio output-mute"` | Action on middle click. Default toggles the speaker's mute. |
| `scroll-up` | [`ClickAction`](/shell/config/types#click-action) | `"lys-shell audio output-volume +5"` | Action on scroll up. Default raises the speaker volume by 5. |
| `scroll-down` | [`ClickAction`](/shell/config/types#click-action) | `"lys-shell audio output-volume -5"` | Action on scroll down. Default lowers the speaker volume by 5. |

::: details More about `left-click`

The `lys-shell audio` commands name their device: `output-mute`
and `output-volume <level>` act on the speaker, `input-mute` and
`input-volume <level>` on the microphone. A level is 0-100, or
`+N` and `-N` to move it.

:::

## Dropdown

| Field | Type | Default | Description |
|---|---|---|---|
| `dropdown-app-icons` | [`AppIconSource`](/shell/config/types#app-icon-source) | `"mapped"` | Icon source for app volume entries in the audio dropdown. |

## Default configuration

```yaml
modules:
  audio:
    speaker-icons:
      - ld-volume-symbolic
      - ld-volume-1-symbolic
      - ld-volume-2-symbolic
    speaker-icon-muted: ld-volume-x-symbolic
    mic-icon: ld-mic-symbolic
    mic-icon-muted: ld-mic-off-symbolic
    mic-hide-when-idle: false
    capture-camera-icon: ld-webcam-symbolic
    capture-screen-icon: ld-monitor-symbolic
    capture-color: status-warning
    border-show: false
    border-color: red
    icon-show: true
    tooltip: true
    icon-color: auto
    icon-bg-color: red
    label-show: true
    label-color: fg-default
    format: '{{ percent }}%'
    label-max-length: 0
    button-bg-color: bg-surface-elevated
    left-click: dropdown:audio
    right-click: ""
    middle-click: lys-shell audio output-mute
    scroll-up: lys-shell audio output-volume +5
    scroll-down: lys-shell audio output-volume -5
    dropdown-app-icons: mapped
    thresholds: []
```


</div>
