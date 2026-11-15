# Fedora

Download the `.rpm` of each app you want from the
[latest release](https://github.com/stubbedev/lys/releases/latest). On
x86_64 they end in `.x86_64.rpm`, on aarch64 in `.aarch64.rpm`. The RPMs are
built for the current `fedora:latest`. Install them in one command, so dnf
resolves their dependencies together:

```sh
sudo dnf install ./lys-wm-<version>-*.x86_64.rpm ./lys-shell-<version>-*.x86_64.rpm \
  ./lys-login-<version>-*.x86_64.rpm ./lys-settings-<version>-*.x86_64.rpm \
  ./lys-secrets-<version>-*.x86_64.rpm
```

lys-polkit has no `.deb` or `.rpm`: install it with
`go install github.com/stubbedev/lys/polkit/cmd/lys-polkit@latest`, or through
the flake ([Other systems](/install/other)), and copy its user unit from
`polkit/dist/lys-polkit.service.in` with `@bindir@` replaced.

The packages install binaries, units, PAM services and data files, and enable
nothing.

## Switch the display manager

```sh
sudo systemctl disable gdm   # or your current display manager
sudo systemctl enable lys-login
```

Reboot. The greeter offers the `lys-wm` session. With SELinux enabled,
`/usr/bin/lys-login` is labelled `xdm_exec_t`.
[lys-login on Debian, Ubuntu and Fedora](/login/install/packages) covers the
greeter's config, autologin and how to revert.

## Enable the session services

Disable any other Secret Service provider and polkit agent first, then:

```sh
systemctl --user enable lys-shell lys-secrets lys-polkit
```

They start with your next session. Set the lock screen's PAM service in
`~/.config/lys-shell/config.yaml`; on Fedora the default `system-auth` also
works:

```yaml
login:
  lock:
    pam-service: lys-shell
```

## Boot splash

lys-splash ships a NixOS module and an initramfs-tools hook, but no dracut
module, so it does not run in Fedora's initramfs.

## Dependencies

Each package requires the libraries it loads and recommends the services it
talks to. The app pages list them:
[lys-wm](/wm/guide/install#fedora),
[lys-shell](/shell/guide/getting-started#runtime-dependencies),
[lys-login](/login/install/packages),
[lys-secrets](/secrets/guide/getting-started#debian-ubuntu-fedora).
