---
title: idle
outline: [2, 3]
---

# idle

<div v-pre>

Built-in idle policy: keyboard backlight off, dim, lock, screens off,
suspend, lock before sleep.

Each stage runs after the session has been idle for its timeout, with
separate timeouts on AC and on battery.

Idle inhibitors hold every stage: a video or game holding the
compositor's idle inhibitor, the idle-inhibit module's toggle, a
sandboxed app through the portal, and apps calling
`org.freedesktop.ScreenSaver.Inhibit`. Turn `enable` off to leave idle to
swayidle or hypridle instead.

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `enable` | bool | `true` | Run the built-in idle policy. Turn it off when swayidle or hypridle handles idle, so the two do not both act. |
| `keyboard-backlight` | [`IdleTimeouts`](/shell/config/types#idle-timeouts) | `{...}` | Idle time before every keyboard backlight (`*::kbd_backlight`) turns off. The next key press or other input turns it back on at its level. While lys-power's automatic brightness finds the room bright, the keyboard backlight stays off whatever the input. |
| `dim` | [`IdleTimeouts`](/shell/config/types#idle-timeouts) | `{...}` | Idle time before the backlights dim. |
| `attention` | [`IdleAttention`](/shell/config/types#idle-attention) | `"off"` | Look through the camera for a face turned to the screen before dimming, through lys-power's attention check. A face postpones the dim one dim interval, three times at most without input, and the stages after the dim wait with it; nobody there dims at once. Off, the camera is never used for it. The check is one capture of under a second, the infrared camera first; its frames are reduced to present or absent in memory and never kept, written or sent, and it never runs while the session is locked or another app uses the camera, when the dim happens as without it. |
| `dim-percent` | [`IdleDimPercent`](/shell/config/types#idle-dim-percent) | `30` | Share of its level, in percent, a dimmed backlight keeps. Input restores the exact level it had. |
| `lock` | [`IdleTimeouts`](/shell/config/types#idle-timeouts) | `{...}` | Idle time before the session locks (through logind, which starts the lock screen). |
| `screen-off` | [`IdleTimeouts`](/shell/config/types#idle-timeouts) | `{...}` | Idle time before the outputs power off. Input powers them back on. |
| `suspend` | [`IdleTimeouts`](/shell/config/types#idle-timeouts) | `{...}` | Idle time before the system sleeps. |
| `suspend-action` | [`SleepAction`](/shell/config/types#sleep-action) | `"auto"` | How the suspend stage sleeps, through logind. lys-wm's `lid_action` takes the same values. |
| `lock-before-sleep` | bool | `true` | Lock the session before the system sleeps (a lid close, a suspend from any program), and hold the sleep until the lock screen is up, so the machine always wakes locked. |

## Default configuration

```yaml
idle:
  enable: true
  keyboard-backlight:
    ac: never
    battery: 1m
  dim:
    ac: 4m
    battery: 2m
  attention: off
  dim-percent: 30
  lock:
    ac: 5m
    battery: 3m
  screen-off:
    ac: 10m
    battery: 5m
  suspend:
    ac: never
    battery: 15m
  suspend-action: auto
  lock-before-sleep: true
```


</div>
