# lys-power

lys-power controls when a laptop's battery charges and which power profile it
runs, in one static Go binary. It runs as the `lys-power` system service.

A battery left plugged in at 100% wears out fast. So while the laptop is on
AC, lys-power holds the charge between **75%** and **80%**. It learns at what
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

- Each unplug is recorded with its weekday and minute, in
  `/var/lib/lys-power/unplugs`. The last 60 unplugs are kept.
- On unplug, the 75-80% thresholds come back at once, and a pending `full`
  request is cleared.
- On AC, the charge limit rises to 95-100% when a `full` request is pending,
  or when the predicted unplug is less than 120 minutes away.
- The prediction is the lower quartile of the recorded unplug minutes that are
  still ahead today. Unplugs on the same weekday are used first, and every
  weekday's when the same weekday has fewer than 3. With fewer than 3 records
  ahead there is no prediction, and the charge stays at 75-80%. Once the
  usual unplug time has passed, the charge stays at 75-80% overnight.
- The kernel only exposes `charge_control_start_threshold` and
  `charge_control_end_threshold`. lys-power writes the end threshold first when
  raising them and the start threshold first when lowering them, as embedded
  controllers require, and never rewrites values already in place.

The service wakes only when something can change the result:

- UPower's `OnBattery` property changes (plug or unplug);
- a `full` request appears in `/run/battery-charge`;
- the time the prediction raises or lowers the limit arrives, on a wall-clock
  timer that still fires on time after a suspend;
- the service receives `SIGHUP`.

On battery it sets no timer. Without a system bus it checks the power source
once a minute instead. It writes state only when the state changes, and
atomically.

## Command line

```
lys-power run       Evaluate continuously (the service runs this)
lys-power once      Evaluate once
lys-power full      Charge to 100% until the next unplug
lys-power status    Print the power source, thresholds, profile and prediction
lys-power predict   Print the predicted unplug time
lys-power version   Print the version
```

`full` writes a flag file into `/run/battery-charge`, which members of the
`users` group may write to; the service picks it up at once. `once` suits a
setup that runs lys-power from a timer or a udev rule instead of as a service.

The environment variables `LYS_POWER_SUPPLY_DIR`, `LYS_POWER_STATE_DIR` and
`LYS_POWER_RUNTIME_DIR` replace `/sys/class/power_supply`,
`/var/lib/lys-power` and `/run/battery-charge`.

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
| `services.lys-power.package` | the flake's `lys-power` | The package whose unit and tmpfiles rule are installed. |

The module installs `lys-power`, its system unit, wanted by
`graphical.target`, and the tmpfiles rule that creates `/run/battery-charge`.
It also enables `services.upower` and `services.power-profiles-daemon` with
`mkDefault`. Set either to `false` to run without it, for example when TLP
manages the power profiles.

## Other distributions

The `.deb` and `.rpm` on the
[latest release](https://github.com/stubbedev/lys/releases/latest) install
`/usr/bin/lys-power`, the unit `lys-power.service` and the tmpfiles rule
`lys-power.conf`. They enable nothing:

```sh
sudo systemctl enable --now lys-power
```

The tarball `lys-power-<version>-<arch>-linux.tar.gz` holds the same files.
Install the binary as `/usr/bin/lys-power`, the unit under
`/usr/lib/systemd/system/` and `lys-power.tmpfiles` as
`/usr/lib/tmpfiles.d/lys-power.conf`, then run
`sudo systemd-tmpfiles --create lys-power.conf` and enable the unit.

lys-power needs a battery that exposes `charge_control_*_threshold` (ThinkPad,
ASUS and Framework laptops among others), and `upower` and
`power-profiles-daemon` (or Fedora's `tuned-ppd`) for the profile switching
and the instant reaction to plug and unplug.
