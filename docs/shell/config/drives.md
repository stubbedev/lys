---
title: drives
outline: [2, 3]
---

# drives

<div v-pre>

Removable drives.

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `automount` | bool | `true` | Mount the filesystems on removable drives, such as USB sticks and SD cards, as they are plugged in, and those already plugged in at login, with a notification that opens or ejects each. |

::: details More about `automount`

Internal drives are never mounted this way, and encrypted volumes
stay locked until unlocked in the file manager.

:::

## Default configuration

```yaml
drives:
  automount: true
```


</div>
