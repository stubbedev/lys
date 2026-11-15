<p align="center">
  <img src="../docs/public/lys-splash.svg" width="160" alt="lys-splash">
</p>

<h1 align="center">lys-splash</h1>

<p align="center">
  The boot splash of <a href="../README.md">lys</a>, in place of Plymouth. It starts in the
  initrd, draws on every output, asks for disk passwords, survives the
  switch to the real root and hands its last frame to the display manager.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/splash/">Documentation</a>
</p>

<p align="center">
  <img src="internal/theme/testdata/golden/boot-progress-50.png" width="480" alt="The boot splash">
</p>

It speaks plymouthd's socket protocol and answers to plymouth's command line,
so systemd, its password agents and cryptsetup's scripts work with it
unchanged. It is one static, pure-Go binary with its theme, font and logo
embedded.

## Install

NixOS, with `inputs.lys.url = "github:stubbedev/lys";`:

```nix
{
  imports = [lys.nixosModules.lys-splash];
  boot.lys-splash.enable = true;
}
```

Ubuntu and Debian: copy the package's `compat` output under `/usr`; see the
[install guide](https://stubbedev.github.io/lys/splash/guide/install).

## Use

The next boot shows the splash. Esc toggles the boot messages.
`plymouth.enable=0` on the kernel command line boots without it. The theme is
`/etc/lys-splash/theme.yaml`:

```yaml
accent: "#7aa2f7"
throbber: dots
```

Do not start the daemon on a running desktop: it takes over the display.

## License

MIT. The embedded font is Inter Regular under the SIL Open Font License 1.1
(`internal/theme/assets/LICENSE-Inter.txt`); the logo is lys's.
