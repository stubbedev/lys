# lys-power

lys-power controls when a laptop's battery charges and which power profile it
runs, in one static Go binary. It runs as the `lys-power` system service. As
the `lys-power-brightness` user service it can also set the screen brightness
from the room's light: see [Automatic brightness](#automatic-brightness).

A battery left plugged in at 100% wears out fast. So while the laptop is on
AC, lys-power holds the charge at a limit you choose, **80%** unless you
choose 85, 90, 95 or 100%. It learns at what
time the charger usually comes out and charges to **100%** in the two hours
before then. You leave with a full battery that did not spend the night at
full.

It also switches the
[power-profiles-daemon](https://gitlab.freedesktop.org/upower/power-profiles-daemon)
profile when the power source changes: `performance` on AC and `power-saver`
on battery. When one of those is missing it uses `balanced`. It switches only
when the power source changes, so a profile you pick by hand stays until the
next plug or unplug.

On a machine without a battery, or with an embedded controller that hides the
charge thresholds, only the profile switching applies. Without
power-profiles-daemon, only the charge policy applies.

## The charge policy

- The charge limit is a choice of 80, 85, 90, 95 or 100%, 80% until one is
  chosen, kept in `/var/lib/lys-power/charge-limit`. Charging resumes 5
  points under it, so 80% holds between 75% and 80%. 100% turns the hold
  off. Settings' Power & Battery page chooses it over
  `dev.stubbe.lys.Charge1`.
- Each unplug is recorded with its weekday, its minute and whether the
  laptop was docked during that time on AC, in
  `/var/lib/lys-power/unplugs`. The last 60 unplugs are kept.
- The laptop is docked while an external display is connected (any DRM
  connector other than eDP, LVDS, DSI, writeback or virtual). Without any
  display connector to tell by, the dock state is unknown.
- On unplug, the limit comes back at once, and a pending `full` request is
  cleared.
- On AC the battery charges to full (95-100%):
  - while a `full` request is pending;
  - within 120 minutes of the next calendar event away from the desk, which
    lys-shell passes over the bus: a timed event with a location. lys-power
    keeps only its start time, in `/run/battery-charge/departure`, and an
    event already started opens nothing;
  - on a bare charger, undocked, which counts as travelling;
  - docked, when the unplugs recorded from the dock predict an unplug less
    than 120 minutes away; with the dock state unknown, when any recorded
    unplugs do. Otherwise it holds at the limit.
- The prediction is the lower quartile of the recorded unplug minutes that are
  still ahead today. Unplugs on the same weekday are used first, and every
  weekday's when the same weekday has fewer than 3. With fewer than 3 records
  ahead there is no prediction, and the charge holds at the limit. Once the
  usual unplug time has passed, the charge holds overnight.
- The kernel only exposes `charge_control_start_threshold` and
  `charge_control_end_threshold`. lys-power writes the end threshold first when
  raising them and the start threshold first when lowering them, as embedded
  controllers require, and never rewrites values already in place.
- The policy, why the battery holds or charges to full and when it is meant
  to be full, is the `State` property of `dev.stubbe.lys.Charge1` on the
  system bus, which lys-shell's battery dropdown shows. Its `SetLimit`,
  `ChargeFull` and `SetDeparture` methods need the polkit action
  `dev.stubbe.lys.power.charge`, which an active local session holds.

The service wakes only when something can change the result:

- UPower's `OnBattery` property changes (plug or unplug);
- a `full` request or a departure appears in `/run/battery-charge`;
- the charge limit changes;
- a display is connected or disconnected (a DRM uevent);
- the time the prediction or an event raises or lowers the limit arrives,
  on a wall-clock timer that still fires on time after a suspend;
- the service receives `SIGHUP`.

On battery it sets no timer. Without a system bus it checks the power source
once a minute instead. It writes state only when the state changes, and
atomically.

## Command line

```
lys-power run       Evaluate continuously (the service runs this)
lys-power once      Evaluate once
lys-power full      Charge to 100% until the next unplug
lys-power status    Print the power source, thresholds, battery health and cycles,
                    charge limit, dock, profile, prediction, device power and
                    last sleeps
lys-power predict   Print the predicted unplug time
lys-power energy [-for 10s] [-threads] [-json]
                    Measure what each app costs in power, CPU, wakeups and GPU time
lys-power brightness
                    Set the backlight from the room's light (the user service runs this)
lys-power version   Print the version
```

`full` writes a flag file into `/run/battery-charge`, which members of the
`users` group may write to; the service picks it up at once. `once` suits a
setup that runs lys-power from a timer or a udev rule instead of as a service.

The environment variables `LYS_POWER_SUPPLY_DIR`, `LYS_POWER_STATE_DIR` and
`LYS_POWER_RUNTIME_DIR` replace `/sys/class/power_supply`,
`/var/lib/lys-power` and `/run/battery-charge`. `LYS_POWER_IIO_DIR` and
`LYS_POWER_DEV_DIR` replace `/sys/bus/iio/devices` and `/dev`, where
`brightness` looks for a light sensor and a camera. `LYS_POWER_CGROUP_DIR`,
`LYS_POWER_PROC_DIR` and `LYS_POWER_POWERCAP_DIR` replace `/sys/fs/cgroup`,
`/proc` and `/sys/class/powercap`, where energy attribution reads.
`LYS_POWER_SYS_DIR` replaces `/sys`, where the [device power](#device-power)
policy reads and writes.

## Device power

macOS powers down radios, storage links, PCIe links, USB and audio whenever
they are idle. On Linux most of that is left to TLP or `powertop --auto-tune`.
lys-power applies a small, conservative device policy itself, when it starts
and on every plug and unplug:

| Knob | Which devices | On battery | On AC |
| --- | --- | --- | --- |
| PCI runtime PM (`power/control`) | Wi-Fi (`iwlwifi`, `ath11k_pci`, `ath12k_pci`, `mt7921e`, `mt7925e`), Bluetooth (`btintel_pcie`), NVMe (`nvme`), SMBus (`i801_smbus`) | `auto` | `on` |
| USB autosuspend (`power/control`) | every device except the ones below | `auto` | `on` |
| Wi-Fi power save (nl80211) | every station interface | on | off |
| PCIe ASPM (`pcie_aspm.policy`) | the whole machine | `powersave` | `default` |
| SATA link power (`link_power_management_policy`) | every AHCI host | `med_power_with_dipm` | `max_performance` |
| HD audio power save (`snd_hda_intel.power_save`) | the whole machine | 1 s | 10 s |

It never touches:

- **USB input devices**: anything with a HID interface (keyboards, mice,
  touchpads, receivers), since an autosuspended input device lags and drops
  the first key;
- USB audio devices, printers, Bluetooth adapters, modems and serial devices;
- USB hubs, which the kernel already autosuspends;
- PCI devices whose driver is not on the list above, such as the Intel
  Management Engine and USB controllers that manage their own runtime PM;
- any device on the denylist.

PCIe ASPM stays at `powersave`, never `powersupersave`, whose L1 substates hang
some machines. Where the firmware does not let the kernel change the ASPM
policy, the kernel refuses it and lys-power logs that it kept the firmware's.
NVMe drives save power through APST on their own; lys-power only checks that
APST is enabled and shows it in `lys-power status`.

On a machine without a battery the policy does nothing. While TLP,
`auto-cpufreq` or `tuned` (without `tuned-ppd`) runs, it does nothing either,
and says so in the log and in `lys-power status`. Without the system bus it
cannot rule them out, so it does nothing then too.

Every knob is written only when it is not already at its value, with one
write each, and a failed write is logged with its sysfs path while the other
knobs are still set. A value you set by hand stays until the next plug or
unplug. `lys-power status` lists every knob, its value, the value it is moving
to, and why a knob is or is not managed.

`LYS_POWER_DEVICE_POLICY` chooses the policy: `battery` (the default) powers
devices down on battery and puts everything back at full performance on AC;
`always` also keeps PCI runtime PM and USB autosuspend on while on AC; `off`
leaves every device alone. `LYS_POWER_DEVICE_DENY` lists devices never touched,
separated by spaces, as `pci:vendor:device` or `usb:vendor:product` in hex, as
`lspci -nn` and `lsusb` print them.

## Energy per app

lys-power estimates what each app costs in power, as Activity Monitor's
Energy Impact does on macOS. An app is a systemd unit: every scope and service
in the cgroup tree, such as `app-firefox-….scope` or a terminal's
`tmux-spawn-….scope`, with every process inside it. For each it measures:

- CPU time, the cgroup's `cpu.stat` `usage_usec`;
- wakeups, the context switches of each of its threads;
- GPU time, from the DRM clients its processes hold open
  (`drm-engine-*` or `drm-cycles-*` in `/proc/<pid>/fdinfo`).

Its **impact** folds these into one busy fraction of a CPU, a wakeup costing
50 µs. The machine's power comes from the RAPL package energy counter
(`/sys/class/powercap/intel-rapl:*`, the platform's `psys` zone where there is
one), or else from the battery's discharge power. Each app gets the share of
that power its impact has, the kernel's own CPU time keeping its part. With no
power reading over the whole window, apps carry impact but no watts.

The service publishes the report over the last 5 minutes as the `Report`
property of `dev.stubbe.lys.Energy1` on the system bus, which every user may
read. It samples only when it must: every 2 seconds for 30 seconds after
someone calls `Watch` (lys-shell's battery dropdown does while it is open),
once a minute on battery, and never on AC with nobody watching.

`lys-power energy` makes the same measurement in the terminal, over `-for`,
with each thread's wakeups under `-threads` and as JSON under `-json`. Run it
as root to read the RAPL counters and every process's GPU time.

## Pausing services on battery

A developer laptop often carries services that keep running at full rate on
battery: containers, databases, indexers, maintenance timers. lys-power can
pause the ones you choose while the laptop is on battery and resume them when
it is plugged in again. Nothing is paused until you choose it, in lys-settings
under **Power & Battery**.

- A service or a scope (a Docker or Podman container, for example) is
  **frozen** with the cgroup freezer, through systemd's `FreezeUnit`, the same
  as `systemctl freeze`. It keeps its memory, its open files and its sockets,
  and carries on where it stopped once thawed (`ThawUnit`).
- A timer is **held**: it is stopped, so it does not elapse on battery, and
  started again on AC. A timer with `Persistent=true` then runs what it
  missed.
- Your own units go through your user's systemd manager, at
  `/run/user/<uid>/systemd/private`; system units through the system manager.

Units the session or the system needs are never paused, and cannot be chosen:
systemd's own, D-Bus, logins and sessions, lys, audio, networking, polkit,
UPower, the power profiles, the portals and the apps of your session.

Choosing system units asks for an administrator's password through polkit
(the action `dev.stubbe.lys.power.pause-system-units`); choosing your own
units does not. The choice and what lys-power paused are kept in
`/var/lib/lys-power/pause`, so a restart on battery neither pauses anything
twice nor forgets to resume it on AC.

lys-shell tells you when it paused something, with **Resume for this
session**, which resumes everything until the next unplug, and **Keep running
on battery**, which stops pausing those units. The battery dropdown lists what
is paused. A unit that cannot be paused or resumed is listed with systemd's
error; a failed resume is retried every minute and on every power change
until it succeeds.

The state is the `State` property of `dev.stubbe.lys.Pause1` on the system
bus, which every user may read; `PickSystem`, `PickUser` and `Resume` change
it.
## Timer slack for napping apps

An app the user cannot see naps, as on macOS (see lys-shell's App Nap). Part
of a nap is a larger timer slack, so the app's timers coalesce, and writing
another task's `/proc/<tid>/timerslack_ns` takes `CAP_SYS_NICE`. lys-power
offers it on the system bus as `SetTimerSlack(unit, nanoseconds)` of
`dev.stubbe.lys.Nap1`: for the calling user's own unit (a scope or service in
`user@<uid>.service`), it writes the slack of every thread in the unit's
cgroup that the user owns, and 0 restores the kernel's default. Threads of
other users in that cgroup, such as `sudo`, and other users' units are never
touched, and the slack is at most a second.

## Sleep reports

Every sleep is measured, as `pmset -g log` does on macOS. lys-power holds a
logind delay inhibitor so that, as the system goes to sleep, it reads the
platform's deepest-state residency (the ACPI LPIT counter
`/sys/devices/system/cpu/cpuidle/low_power_idle_system_residency_us`, or
Intel `pmc_core`'s `slp_s0_residency_usec` in debugfs), the battery's energy
and the wakeup counts under `/sys/class/wakeup`; it reads them again as the
system wakes. A sleep of at least ten minutes that spent less than 90% of its
time in the deepest state is shallow; one that started and ended on battery
has its drain, the energy it used divided by the time asleep. The devices that
woke the system are the wakeup sources that fired during the sleep, the most
often first; on Intel platforms the elements `pmc_core`'s
`substate_requirements` reports required and unmet are what kept it shallow.
A sleep that resumed from hibernation, whose counter started over, or a
platform without a counter, is not judged.

The last ten sleeps are kept in `/var/lib/lys-power/sleeps`, printed by
`lys-power status`, and published as the `Sleeps` property of
`dev.stubbe.lys.Sleep1` on the system bus, which every user may read.
lys-shell notifies once after each shallow sleep on battery with its drain
and what kept it shallow or woke it; a good sleep raises nothing.

## Wake sources

`lys-power status` lists what may wake the machine: the ACPI devices
enabled in `/proc/acpi/wakeup` and the USB devices whose `power/wakeup` is
enabled. With `LYS_POWER_USB_WAKE=battery` lys-power disables wakeup on
every USB device the firmware places on a removable port while on battery,
since a mouse bumped in a bag wakes a closed laptop, and enables it again on
AC for the devices it disabled. Hubs, whose wakeup carries that of the
built-in keyboard and touchpad behind them, and devices on fixed or
undescribed ports keep theirs. The default, `keep`, leaves every device
alone.

## Automatic brightness

Automatic brightness sets the backlight from the light in the room. It is off
unless you enable it: running the `lys-power-brightness` user service is what
turns it on. It runs in your graphical session, as you, not as root.

**Where the light comes from.** An ambient light sensor is preferred: the
first IIO device under `/sys/bus/iio/devices` with `in_illuminance_input`, or
`in_illuminance_raw` with its `in_illuminance_scale` and
`in_illuminance_offset`. It is read every 2 seconds. Without one, the camera
estimates the light: the first `/dev/video*` capture node with manual
exposure control. With neither, the service exits with an error that names
what it looked for, and the unit shows as failed.

**The camera.** Once a minute, lys-power switches the camera on at the
smallest frame size it offers, for about a second. A camera's auto exposure
makes every picture look about equally bright, so the brightness of a frame
alone says little about the room. lys-power lets auto exposure settle, then
holds the exposure manually at the camera's default gain and adjusts it until a
frame is neither too dark nor too bright, and takes the frame's brightness
divided by its exposure time as the room's light. Afterwards it puts the
exposure and gain controls back as they were. If another application is
using the camera, for example during a call, that sample is skipped and the
backlight stays where it is.

**Privacy.** A frame is only averaged into a single brightness number, in
memory. No frame is ever written to disk, kept or sent anywhere. While the
backlight is paused, or the session is locked or in the background, the camera
is not switched on at all.

**How the backlight moves.** The light is smoothed with a moving average over
about 30 seconds. The level follows the logarithm of the illuminance, from 5%
at 1 lux to 100% at 3000 lux. The backlight moves only when the new level is
more than 6 percentage points from the current one, and then in steps over
1.5 seconds rather than in one jump. It drives every backlight under
`/sys/class/backlight`, through logind, as the shell's brightness keys do.

**Low Power Mode.** While power-profiles-daemon's profile is `power-saver`
(which lys-power picks on battery), the level for the room is 12% lower, as
macOS slightly dims the display on battery. The backlight moves there, and
back when the profile changes again, in the same gradual steps, as automatic
brightness's own change: it does not pause it. A profile change during the
idle dim takes effect once the shell has put the screen back.

**Pausing and resuming.** When the backlight changes to a level lys-power did
not set, from the shell's brightness slider, a brightness key or any other
program, automatic brightness pauses and leaves the backlight alone. The
shell's brightness widget shows whether automatic brightness is following the
room or paused, and its "follow the room again" action hands control back: the
backlight then goes to the level for the current light at once. The idle dim
does not count as a change: the shell marks the session idle in logind (its
`IdleHint`) before it dims the screen after a period of inactivity and clears
the mark once it has put the screen back when you return, so automatic
brightness carries on. Lowering the backlight yourself and raising it back to
exactly where it was keeps it paused, as does changing a dim yourself before
it is put back. lys-power neither samples nor writes while the session is
locked or another session is in the foreground. When your display session ends, or another of your sessions
becomes it, the service exits with an error naming the session, and systemd
starts it again for the session now in front.

**The room.** Above 500 lux (bright daylight indoors) the room counts as
bright, and below 250 lux as dim again, so a room near one threshold does
not flip back and forth. The shell keeps the keyboard backlight off in a
bright room. While automatic brightness is paused it samples nothing, so
the room is unknown.

The state is on the session bus as the `State` property (`following` or
`paused`) of `dev.stubbe.lys.AutoBrightness1` at
`/dev/stubbe/lys/AutoBrightness`, the room as its `Room` property
(`unknown`, `dim` or `bright`), with `PropertiesChanged` on every change of
either, and `Resume()` hands control back:

```sh
busctl --user get-property dev.stubbe.lys.AutoBrightness1 /dev/stubbe/lys/AutoBrightness dev.stubbe.lys.AutoBrightness1 State
busctl --user call dev.stubbe.lys.AutoBrightness1 /dev/stubbe/lys/AutoBrightness dev.stubbe.lys.AutoBrightness1 Resume
```

The name is on the bus only while the service runs.

On NixOS, set `services.lys-power.autoBrightness.enable = true;` (see
[NixOS](#nixos)). On other distributions, the packages install the user unit
without enabling it:

```sh
systemctl --user enable --now lys-power-brightness
```

Your user needs access to the camera, which logind grants the user at the
seat, and the backlight is written through logind's `SetBrightness`, so no
group membership is needed.

## Upgrading from adaptive-power-manager

On its first start with no unplug history, lys-power copies the history of
adaptive-power-manager from `/var/lib/adaptive-power-manager/unplugs`, or
else that of the older bash policy from `/var/lib/power-source/unplugs`, so
the prediction does not start from nothing. Disable
`adaptive-power-manager.service` before enabling lys-power: two services
writing the same thresholds work against each other.

## NixOS

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  # in the NixOS configuration
  imports = [inputs.lys.nixosModules.lys-power];
  services.lys-power.enable = true;
}
```

| Option | Default | Description |
| --- | --- | --- |
| `services.lys-power.enable` | `false` | Run the `lys-power` system service. |
| `services.lys-power.package` | the flake's `lys-power` | The package whose units and tmpfiles rule are installed. |
| `services.lys-power.autoBrightness.enable` | `false` | Start the `lys-power-brightness` user service, [automatic brightness](#automatic-brightness), with every graphical session. |
| `services.lys-power.devicePower.policy` | `"battery"` | When idle devices power down, `"battery"`, `"always"` or `"off"`: see [device power](#device-power). |
| `services.lys-power.devicePower.deny` | `[]` | Devices the device power policy never touches, such as `"usb:046d:c52b"`. |
| `services.lys-power.usbWake` | `"keep"` | `"battery"` keeps removable USB devices from waking the machine on battery: see [wake sources](#wake-sources). |

The module installs `lys-power`, its system unit, wanted by
`graphical.target`, and the tmpfiles rule that creates `/run/battery-charge`.
It also enables `services.upower`, `services.power-profiles-daemon` and
`security.polkit` with `mkDefault`. Set any of them to `false` to run without
it, for example when TLP manages the power profiles. `autoBrightness.enable` installs the package's
user unit `lys-power-brightness.service` and has it wanted by
`graphical-session.target`; it works with or without `enable`.

## Other distributions

The `.deb` and `.rpm` on the
[latest release](https://github.com/stubbedev/lys/releases/latest) install
`/usr/bin/lys-power`, the unit `lys-power.service`, the user unit
`lys-power-brightness.service`, the tmpfiles rule `lys-power.conf`, the
system bus policies `dev.stubbe.lys.Energy1.conf` and
`dev.stubbe.lys.Pause1.conf`, and the polkit action
`dev.stubbe.lys.power.policy`. They enable nothing:
`lys-power-brightness.service`, the tmpfiles rule `lys-power.conf` and the
`dev.stubbe.lys.Nap1.conf` and `dev.stubbe.lys.Sleep1.conf`. They enable nothing:

```sh
sudo systemctl enable --now lys-power
systemctl --user enable --now lys-power-brightness   # automatic brightness, optional
```

The tarball `lys-power-<version>-<arch>-linux.tar.gz` holds the same files.
Install the binary as `/usr/bin/lys-power`, the unit under
`/usr/lib/systemd/system/`, `lys-power-brightness.service` under
`/usr/lib/systemd/user/` and `lys-power.tmpfiles` as
`/usr/lib/tmpfiles.d/lys-power.conf`, `dev.stubbe.lys.Energy1.conf` and
`dev.stubbe.lys.Pause1.conf` under `/usr/share/dbus-1/system.d/` and
`dev.stubbe.lys.power.policy` under `/usr/share/polkit-1/actions/`, then run
`/usr/lib/tmpfiles.d/lys-power.conf` and `dev.stubbe.lys.Energy1.conf` and
`dev.stubbe.lys.Nap1.conf` and `dev.stubbe.lys.Sleep1.conf` under
`/usr/share/dbus-1/system.d/`, then run
`sudo systemd-tmpfiles --create lys-power.conf` and enable the unit.

lys-power needs a battery that exposes `charge_control_*_threshold` (ThinkPad,
ASUS and Framework laptops among others), and `upower` and
`power-profiles-daemon` (or Fedora's `tuned-ppd`) for the profile switching
and the instant reaction to plug and unplug.
