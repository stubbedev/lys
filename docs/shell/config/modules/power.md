---
title: power
outline: [2, 3]
---

# power

<div v-pre>

Shutdown, reboot, and logout menu.

Add it to your layout with `power`:

```yaml
bar:
  layout:
    - monitor: "*"
      right: [power]
```

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `icon-name` | string | `"ld-power-symbolic"` | Icon name to display. |
| `border-show` | bool | `false` | Display border around button. |
| `lock-command` | [`Command`](/shell/config/types#command) | `":lock"` | What the power menu's Lock button runs. The default `:lock` asks logind to lock the session, which the lock screen answers; any other string is a shell command line. |
| `logout-command` | [`Command`](/shell/config/types#command) | `":logout"` | What the power menu's Log out button runs. The default `:logout` ends the session through lys-wm, else logind; any other string is a shell command line. |
| `suspend-command` | [`Command`](/shell/config/types#command) | `":suspend"` | What the power menu's Suspend button runs. The default `:suspend` asks logind to suspend; any other string is a shell command line. |
| `reboot-command` | [`Command`](/shell/config/types#command) | `":reboot"` | What the power menu's Reboot button runs. The default `:reboot` asks logind to reboot; any other string is a shell command line. |
| `shutdown-command` | [`Command`](/shell/config/types#command) | `":poweroff"` | What the power menu's Shut down button runs. The default `:poweroff` asks logind to power off; any other string is a shell command line. |
| `show-lock` | bool | `true` | Show the Lock button in the power menu. |
| `show-logout` | bool | `true` | Show the Log out button in the power menu. |
| `show-suspend` | bool | `true` | Show the Suspend button in the power menu. |
| `show-reboot` | bool | `true` | Show the Reboot button in the power menu. |
| `show-shutdown` | bool | `true` | Show the Shut down button in the power menu. |

## Colors

| Field | Type | Default | Description |
|---|---|---|---|
| `border-color` | [`ColorValue`](/shell/config/types#color-value) | `"red"` | Border color token. |
| `icon-color` | [`ColorValue`](/shell/config/types#color-value) | `"auto"` | Icon foreground color. Auto selects based on variant for contrast. |
| `icon-bg-color` | [`ColorValue`](/shell/config/types#color-value) | `"red"` | Icon container background color token. |

## Click actions

| Field | Type | Default | Description |
|---|---|---|---|
| `right-click` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on right click. |
| `middle-click` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on middle click. |
| `scroll-up` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on scroll up. |
| `scroll-down` | [`ClickAction`](/shell/config/types#click-action) | `""` | Action on scroll down. |
| `left-click` | [`ClickAction`](/shell/config/types#click-action) | `":menu"` | Action on left click. Default opens lys-shell's native power menu (`:menu`). |

## Default configuration

```yaml
modules:
  power:
    icon-name: ld-power-symbolic
    border-show: false
    border-color: red
    icon-color: auto
    icon-bg-color: red
    right-click: ""
    middle-click: ""
    scroll-up: ""
    scroll-down: ""
    left-click: :menu
    lock-command: :lock
    logout-command: :logout
    suspend-command: :suspend
    reboot-command: :reboot
    shutdown-command: :poweroff
    show-lock: true
    show-logout: true
    show-suspend: true
    show-reboot: true
    show-shutdown: true
```


</div>
