---
title: thresholds
outline: [2, 3]
---

# thresholds

<div v-pre>

Warning levels of CPU, RAM, disk and battery, shared by their bar
modules and the dashboard.

Each resource is a list of thresholds over its percentage. Entries are
checked in order; the last matching entry wins for each color slot.
The bar module recolors its button with the slots; the dashboard's
gauge for the resource takes the first color an entry sets, in the
order icon, label, button background, icon background, border, and
its battery card colors its icon with `icon-color` and its text with
`label-color`.

## Example

```yaml
thresholds: {cpu: [{above: 70, icon-color: status-warning}, {above: 90, icon-color: status-error}]}
```

## General

| Field | Type | Default | Description |
|---|---|---|---|
| `cpu` | array of [`ThresholdEntry`](/shell/config/types#threshold-entry) | `[...]` | CPU usage percentage. |
| `ram` | array of [`ThresholdEntry`](/shell/config/types#threshold-entry) | `[...]` | RAM usage percentage. |
| `storage` | array of [`ThresholdEntry`](/shell/config/types#threshold-entry) | `[...]` | Disk usage percentage of the storage module's mount, and of the root file system on the dashboard. |
| `battery` | array of [`ThresholdEntry`](/shell/config/types#threshold-entry) | `[...]` | Battery charge percentage. Use `below` for low-charge warnings. |


</div>
