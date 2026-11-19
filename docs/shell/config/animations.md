---
title: animations
outline: [2, 3]
---

# animations

<div v-pre>

Enter/exit and change animations for transient surfaces.

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `enabled` | bool | `true` | Enable enter/exit animations (OSD, toasts, notifications) and icon-change crossfades. When disabled, surfaces appear instantly. |
| `reduced-motion` | bool | `false` | Replace motion with fades in the shell and, through the portal's reduced-motion setting, in apps: surfaces fade instead of sliding, swinging or bouncing, and looping indicators stop. Under lys-wm this key has no effect and says so: lys-wm's `animations.reduced_motion` owns the setting for the whole desktop. |
| `duration` | [`AnimationMS`](/shell/config/types#animation-m-s) | `200` | Animation duration in milliseconds. |
| `transition` | [`AnimationType`](/shell/config/types#animation-type) | `"fade"` | Transition style used for enter/exit of the OSD, toasts, and notification cards. Base fallback for every surface and direction. |
| `enter` | [`AnimationType`](/shell/config/types#animation-type) or null | `null` | Global enter transition. Unset → `transition`. |
| `exit` | [`AnimationType`](/shell/config/types#animation-type) or null | `null` | Global exit transition. Unset → `transition`. |
| `enter-duration` | [`AnimationMS`](/shell/config/types#animation-m-s) or null | `null` | Global enter duration in ms. Unset → `duration`. |
| `exit-duration` | [`AnimationMS`](/shell/config/types#animation-m-s) or null | `null` | Global exit duration in ms. Unset → `duration`. |
| `interaction-duration` | [`AnimationMS`](/shell/config/types#animation-m-s) | `150` | Duration in ms for in-place dropdown transitions: hover highlights and the page/stack crossfades inside dropdowns. `enabled: false` removes them entirely. |
| `ui-duration` | [`AnimationMS`](/shell/config/types#animation-m-s) | `250` | Base duration in ms for general UI micro-transitions (hover, focus, and color fades) driven by the CSS `--duration-*` token family. Fast, normal, and slow speeds are derived from this. `enabled: false` zeroes them for an instant UI. |
| `indicators` | bool | `true` | Run looping status indicators: spinners, network/bluetooth scan animations, the recording pulse, and the clock blink. Disable (or set `enabled: false`) for a fully static UI. |
| `notifications` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for notification popup cards. |
| `osd` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the OSD (volume/brightness/toggle/toast). |
| `dropdown` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for bar widget dropdown foldouts. |
| `share-picker` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the screen-share picker overlay. |
| `wallpaper` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the wallpaper crossfade between images. |
| `power` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the power menu overlay. |
| `dialog` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for portal dialogs (access/account/app-chooser/ launcher-install prompts). |
| `file-chooser` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the portal file-chooser surface. |
| `print` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the portal print surface. |
| `lock` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the lock screen reveal. |
| `launcher` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for the application launcher. |
| `menu` | [`SurfaceAnimation`](/shell/config/types#surface-animation) | `{...}` | Per-surface override for menus, context menus, the lists of dropdown inputs and window previews. Enter and exit durations default to 0, so they open in the frame of the click; a longer one starts already showing. `fade` only fades, any other transition also grows the menu out of what it opened from. |

## Default configuration

```yaml
animations:
  enabled: true
  reduced-motion: false
  duration: 200
  transition: fade
  interaction-duration: 150
  ui-duration: 250
  indicators: true
```


</div>
