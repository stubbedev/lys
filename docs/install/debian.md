# Debian and Ubuntu

Download the `.deb` of each app you want from the
[latest release](https://github.com/stubbedev/lys/releases/latest). On
x86_64 they end in `_amd64.deb`, on aarch64 in `_arm64.deb`. Install them in
one command, so apt resolves their dependencies together:

```sh
sudo apt install ./lys-wm_<version>_amd64.deb ./lys-shell_<version>_amd64.deb \
  ./lys-login_<version>_amd64.deb ./lys-settings_<version>_amd64.deb \
  ./lys-secrets_<version>_amd64.deb
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

Reboot. The greeter offers the `lys-wm` session.
[lys-login on Debian, Ubuntu and Fedora](/login/install/packages) covers the
greeter's config, autologin and how to revert.

## Enable the session services

Disable any other Secret Service provider and polkit agent first, then:

```sh
systemctl --user enable lys-shell lys-secrets lys-polkit
```

They start with your next session. Set the lock screen's PAM service in
`~/.config/lys-shell/config.yaml`:

```yaml
login:
  lock:
    pam-service: lys-shell
```

## Boot splash

lys-splash is installed from its Nix build on Debian and Ubuntu, since it
replaces Plymouth in the initramfs: see
[lys-splash's install guide](/splash/guide/install#ubuntu-and-debian).

## Dependencies

Each package depends on the libraries it loads and recommends the services it
talks to. The app pages list them:
[lys-wm](/wm/guide/install#debian-and-ubuntu),
[lys-shell](/shell/guide/getting-started#runtime-dependencies),
[lys-login](/login/install/packages),
[lys-secrets](/secrets/guide/getting-started#debian-ubuntu-fedora).
