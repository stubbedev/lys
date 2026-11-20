---
title: workspaces
outline: [2, 3]
---

# workspaces

<div v-pre>

Workspace indicators with click-to-switch, on whichever compositor runs.

Add it to your layout with `workspaces`:

```yaml
bar:
  layout:
    - monitor: "*"
      right: [workspaces]
```

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `display-mode` | [`DisplayMode`](/shell/config/types#display-mode) | `"label"` | What identifies each workspace button. |
| `urgent-show` | bool | `true` | Pulse animation on workspaces with urgent windows. |
| `urgent-mode` | [`UrgentMode`](/shell/config/types#urgent-mode) | `"workspace"` | Where the urgent pulse is applied. |
| `window-previews` | bool | `false` | Preview a workspace's windows when the pointer rests on its button. |
| `active-indicator` | [`ActiveIndicator`](/shell/config/types#active-indicator) | `"background"` | Visual indicator for the active workspace. |
| `divider` | string | `" "` | Text separator between workspace identity and app icons. |
| `app-icons-show` | bool | `false` | Show application icons for windows on each workspace. |
| `app-icons-dedupe` | bool | `true` | Deduplicate application icons within a workspace. |
| `app-icons-fallback` | string | `"ld-app-window-symbolic"` | Fallback icon for applications not matched by `app-icon-map`. |
| `app-icons-empty` | string | `"ld-minus-symbolic"` | Icon shown when a workspace has no application windows. |
| `icon-gap` | [`Size`](/shell/config/types#size) | `0.3` | Gap between app icons within a workspace button. Accepts a scale multiplier or pixels (e.g. `"4px"`). |
| `workspace-padding` | [`Size`](/shell/config/types#size) | `0.5` | Padding along the bar direction for content wider than the circle: app icons, a divider or a long name. Accepts a scale multiplier or pixels (e.g. `"8px"`). |
| `icon-size` | [`Size`](/shell/config/types#size) | `1` | Workspace icon size. Accepts a scale multiplier or pixels (e.g. `"16px"`). |
| `label-size` | [`Size`](/shell/config/types#size) | `1` | Workspace label and divider size. Accepts a scale multiplier or pixels (e.g. `"16px"`). |
| `active-color` | [`ColorValue`](/shell/config/types#color-value) | `"accent"` | Color for the active (visible on its output) workspace. |
| `occupied-color` | [`ColorValue`](/shell/config/types#color-value) | `"fg-muted"` | Color for occupied workspaces (have windows but not active). |
| `empty-color` | [`ColorValue`](/shell/config/types#color-value) | `"fg-subtle"` | Color for empty workspaces and placeholder slots. |
| `container-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"bg-surface-elevated"` | Background color for the workspaces container. |
| `border-show` | bool | `false` | Display border around the workspaces container. |
| `border-color` | [`ColorValue`](/shell/config/types#color-value) | `"border-default"` | Border color for the workspaces container. |
| `workspace-map` | [`WorkspaceMap`](/shell/config/types#workspace-map) | `{}` | Per-workspace icon and color overrides, keyed by name or id-as-string. |
| `app-icon-map` | map of string | `{}` | Application icon mapping with glob pattern support. |
| `left-click` | [`WorkspaceClickAction`](/shell/config/types#workspace-click-action) | `"focus:this"` | Action on left click. |
| `middle-click` | [`WorkspaceClickAction`](/shell/config/types#workspace-click-action) | `""` | Action on middle click. |
| `right-click` | [`WorkspaceClickAction`](/shell/config/types#workspace-click-action) | `""` | Action on right click. |
| `scroll-up` | [`WorkspaceClickAction`](/shell/config/types#workspace-click-action) | `"focus:previous"` | Action on scroll up. |
| `scroll-down` | [`WorkspaceClickAction`](/shell/config/types#workspace-click-action) | `"focus:next"` | Action on scroll down. |
| `monitor-specific` | bool | `true` | Show only workspaces on this bar's monitor (all but mango). |
| `workspace-ignore` | array of string | `[]` | Workspaces to hide from the display (all but mango). |
| `hide-trailing-empty` | bool | `true` | Hide the trailing empty workspace niri keeps (niri, sway). |
| `label-strategy` | [`LabelStrategy`](/shell/config/types#label-strategy) | `"name-or-index"` | How to compose the workspace label when `display-mode: label` (niri, sway). |
| `min-workspace-count` | u8 | `0` | Minimum number of workspace buttons to display (Hyprland). |
| `show-special` | bool | `true` | Include special workspaces (scratchpads) in the display (Hyprland). |
| `label-use-name` | bool | `false` | Use workspace name instead of number when displaying labels (Hyprland). |
| `numbering` | [`Numbering`](/shell/config/types#numbering) | `"absolute"` | How workspace numbers are displayed (Hyprland). |
| `highlight-active-on-other-monitor` | bool | `true` | Highlight workspaces active on other monitors with a different color (Hyprland). |
| `active-on-other-monitor-color` | [`ColorValue`](/shell/config/types#color-value) | `"accent"` | Active on other monitor indicator color (Hyprland). |
| `hide-empty` | bool | `true` | Hide tags that hold no clients and are not active (mango). |
| `min-tag-count` | u8 | `0` | Always show tags up to this one-based index, even when empty (mango). |

::: details More about `display-mode`

- `label` (default): show the workspace label per `label-strategy`
- `icon`: show an icon from `workspace-map` (falls back to label if unmapped)
- `none`: show nothing — only app icons visible (if enabled)

:::

::: details More about `urgent-mode`

- `workspace` (default): whole button pulses
- `application`: only the urgent app icon pulses, falling back to
  `workspace` when app icons are disabled

:::

::: details More about `window-previews`

When `false` (default), hovering a workspace does nothing and no
window is captured.

:::

::: details More about `app-icons-dedupe`

When `true`, one icon per unique `app_id`. When `false`, one icon
per window.

:::

::: details More about `workspace-padding`

A short label takes no padding. It sets the circle's size instead: at least a digit's width plus this padding on each side.

:::

::: details More about `icon-size`

Applies to identity icons and custom icons from `workspace-map`.

:::

::: details More about `active-color`

In `background` indicator mode, also used as the button background.

:::

::: details More about `workspace-map`

#### Example

```yaml
modules:
  workspaces:
    workspace-map:
      web: {icon: ld-globe-symbolic, color: "#4a90d9"}
      terminal: {icon: ld-terminal-symbolic}
```

:::

::: details More about `app-icon-map`

Maps window `app_id` or title to symbolic icon names. Supports:
- No prefix: matches `app_id` (e.g. `"*firefox*"`)
- `app:` prefix: explicit `app_id` match (e.g. `"app:org.mozilla.*"`)
- `title:` prefix: matches window title (e.g. `"title:*YouTube*"`)

#### Example

```yaml
modules:
  workspaces:
    app-icon-map:
      "*firefox*": ld-globe-symbolic
      "title:*YouTube*": ld-youtube-symbolic
```

:::

::: details More about `monitor-specific`

When `true` (default), each bar shows only its own output's
workspaces. When `false`, all workspaces from every output are shown.

:::

::: details More about `workspace-ignore`

Glob patterns matched against the workspace's name (if set), then
its index, then its stable id. Examples:
- `"scratch"` — hide the workspace named `scratch`
- `"1?"` — hide indices 10-19

:::

::: details More about `hide-trailing-empty`

Niri keeps one empty workspace at the tail of every output for
dynamic allocation.

:::

::: details More about `label-strategy`

- `index`: index only (`"1"`, `"2"`)
- `name-or-index` (default): name when set, index otherwise
- `name-only`: name only; unnamed workspaces show nothing
- `index-and-name`: `"1: web"` form; unnamed workspaces show the index alone

:::

::: details More about `min-workspace-count`

When set to 0 (default), only active and occupied workspaces are shown.
When set to N, at least N buttons are always visible, with empty ones
using `empty-color` styling.

:::

::: details More about `show-special`

Special workspaces have negative IDs in Hyprland.

:::

::: details More about `label-use-name`

Only applies when `display-mode: label` or as fallback for unmapped
workspaces in `display-mode: icon`.

:::

::: details More about `numbering`

- `absolute`: Show actual Hyprland workspace IDs (1, 2, 3, 4, 5, 6...)
- `relative`: Show numbers relative to monitor's starting workspace.
  If a monitor has workspaces 4, 5, 6 assigned, they display as 1, 2, 3.
  Useful when keybinds use per-monitor numbering.

:::

::: details More about `highlight-active-on-other-monitor`

When true, workspaces active on a different monitor are highlighted differently.
When false, workspaces active on another monitor are not specially highlighted.

This setting only makes sense when `monitor-specific` is false.

:::

::: details More about `active-on-other-monitor-color`

Only applies when `highlight-active-on-other-monitor` is `true`.

:::

::: details More about `min-tag-count`

`0` shows only occupied or active tags (subject to `hide-empty`). A
value above the compositor's tag count just shows every tag.

:::

## Default configuration

```yaml
modules:
  workspaces:
    display-mode: label
    urgent-show: true
    urgent-mode: workspace
    window-previews: false
    active-indicator: background
    divider: ' '
    app-icons-show: false
    app-icons-dedupe: true
    app-icons-fallback: ld-app-window-symbolic
    app-icons-empty: ld-minus-symbolic
    icon-gap: 0.30000001192092896
    workspace-padding: 0.5
    icon-size: 1.0
    label-size: 1.0
    active-color: accent
    occupied-color: fg-muted
    empty-color: fg-subtle
    container-bg-color: bg-surface-elevated
    border-show: false
    border-color: border-default
    workspace-map: {}
    app-icon-map: {}
    left-click: focus:this
    middle-click: ""
    right-click: ""
    scroll-up: focus:previous
    scroll-down: focus:next
    monitor-specific: true
    workspace-ignore: []
    hide-trailing-empty: true
    label-strategy: name-or-index
    min-workspace-count: 0
    show-special: true
    label-use-name: false
    numbering: absolute
    highlight-active-on-other-monitor: true
    active-on-other-monitor-color: accent
    hide-empty: true
    min-tag-count: 0
```


</div>
