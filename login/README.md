<p align="center">
  <img src="../docs/public/lys-login.svg" width="160" alt="lys-login">
</p>

<h1 align="center">lys-login</h1>

<p align="center">
  The display manager, greeter and lock screen of <a href="../README.md">lys</a>, in pure Go.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/login/">Documentation</a>
</p>

| Command | What it is |
| --- | --- |
| `lys-login daemon` | The display manager: a root service that shows the greeter in a lys-wm kiosk and starts sessions through PAM. |
| `lys-login greet` | The greeter, in the lys-wm kiosk of `lys-login daemon` or of `lys-login kiosk` under greetd. |
| `lys-login kiosk` | greetd's greeter command: lys-wm with the display manager's kiosk config, hosting `lys-login greet`. |
| `lys-login lock` | The session lock on `ext-session-lock-v1`, started by [lys-shell](../shell). |
| `lys-login upgrade` | Moves the running display manager to a new build without ending sessions. |

Both screens read the `login` section of the lys-shell config. A successful login
or unlock opens the login collection of
[lys-secrets](../secrets), else gnome-keyring.

## Install

The manual's [install section](https://stubbedev.github.io/lys/install/) covers
NixOS (`lys.nixosModules.lys-login` with
`programs.lys-login.greeter.enable = true`), the Debian, Ubuntu and Fedora
packages, the tarball and the greeter under greetd. lys-login's own pages cover
[NixOS](https://stubbedev.github.io/lys/login/install/nixos),
[the packages](https://stubbedev.github.io/lys/login/install/packages) and
[other systems](https://stubbedev.github.io/lys/login/install/other), then
configuration, PAM setup and architecture.

## License

MIT. See [LICENSE](LICENSE).
