<h1 align="center">lys-power</h1>

<p align="center">
  Adaptive battery charging and power profile switching for <a href="../README.md">lys</a>.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/power/">Documentation</a>
</p>

A battery kept plugged in at 100% wears out fast. While the laptop is on AC,
lys-power holds the charge at a limit you choose (80% unless you pick 85, 90,
95 or 100%), learns when the charger usually comes out, and charges to 100% in
the two hours before that moment, before a calendar event away from the desk,
or on a bare charger away from a dock. It also
sets the power profile itself, the ACPI platform profile and the CPUs' energy
preference, on every AC edge: `performance` on AC, `power-saver` on battery,
and refuses to run beside power-profiles-daemon or `tuned-ppd`.
On battery it powers down idle devices, as macOS does: PCI runtime power
management, USB autosuspend (never for keyboards, mice or audio), Wi-Fi power
save, PCIe ASPM, SATA link power and audio power save. Optionally, as a user service, it sets the screen brightness from an ambient
light sensor or short camera captures that are never kept or sent anywhere.

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
system unit; enable it with `systemctl enable --now lys-power`. Automatic
brightness is `services.lys-power.autoBrightness.enable = true;` on NixOS and
`systemctl --user enable --now lys-power-brightness` elsewhere.

## Usage

```sh
lys-power status    # the power source, thresholds, profile, prediction, device power and last sleeps
lys-power full      # charge to 100% until the next unplug
lys-power predict   # the predicted unplug time
lys-power energy    # what each app costs in power, CPU, wakeups and GPU time
```

It runs as the `lys-power` system service, as root, since the charge
thresholds in `/sys/class/power_supply` are root's to write. The
[manual](https://stubbedev.github.io/lys/power/) describes the policy.
