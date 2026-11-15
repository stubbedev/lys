<h1 align="center">lys-power</h1>

<p align="center">
  Adaptive battery charging and power profile switching for <a href="../README.md">lys</a>.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/power/">Documentation</a>
</p>

A battery kept plugged in at 100% wears out fast. While the laptop is on AC,
lys-power holds the charge between 75% and 80%, learns when the charger usually
comes out, and charges to 100% in the two hours before that moment. It also
switches the [power-profiles-daemon](https://gitlab.freedesktop.org/upower/power-profiles-daemon)
profile on every AC edge: `performance` on AC, `power-saver` on battery.

## Install

On NixOS:

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  # in the NixOS configuration
  imports = [inputs.lys.nixosModules.lys-power];
  services.lys-power.enable = true;
}
```

The `.deb` and `.rpm` on the
[latest release](https://github.com/stubbedev/lys/releases/latest) install the
system unit; enable it with `systemctl enable --now lys-power`.

## Usage

```sh
lys-power status    # the power source, thresholds, profile and prediction
lys-power full      # charge to 100% until the next unplug
lys-power predict   # the predicted unplug time
```

It runs as the `lys-power` system service, as root, since the charge
thresholds in `/sys/class/power_supply` are root's to write. The
[manual](https://stubbedev.github.io/lys/power/) describes the policy.
