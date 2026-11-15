# PAM services

lys-login has no authentication of its own. Both screens run a PAM conversation
and show whatever PAM asks.

| Service | Used by | PAM calls |
| --- | --- | --- |
| `lys-login` (`--service`) | Each login through the daemon | authenticate, account, credentials, open session |
| `lys-login` (`--service`) | The greeter's own session | credentials, open session |
| `lys-login-autologin` | [Autologin](#autologin) | as `lys-login`, authenticating with `pam_permit` |
| `lys-login-fingerprint` | A [fingerprint login](#fingerprint) | as `lys-login-autologin` |
| `login.lock.pam-service` | The lock screen | authenticate, account |

Under greetd, greetd authenticates with its own service.

The Debian/Ubuntu and Fedora packages install `lys-login`, `lys-login-autologin`,
`lys-login-fingerprint` and `lys-shell` into `/etc/pam.d`, from
[`resources/pam/`](https://github.com/stubbedev/lys/tree/main/login/resources/pam).
The NixOS module declares them.

## The display manager's service

It needs a session stack with `pam_systemd`, which registers the greeter and each
login with logind. The workers set in the PAM environment:

| Variable | Value |
| --- | --- |
| `XDG_SESSION_TYPE` | `wayland` for the greeter and Wayland sessions, `x11` for X11 sessions; a custom session is `wayland` unless `--env` overrides it |
| `XDG_SESSION_CLASS` | `greeter` or `user` |
| `XDG_SEAT` | `seat0` |
| `XDG_VTNR` | The session's VT |

- **Debian, Ubuntu**: `pam_unix` for the password, then `common-account`,
  `common-session` and `common-password`. The `auth` lines are written out
  instead of including `common-auth`, which `pam-auth-update` gives
  `pam_fprintd` once `libpam-fprintd` is installed (see
  [Fingerprint](#fingerprint)). An account from LDAP, SSSD or Kerberos needs
  its module's `auth` line added after `pam_unix`.
- **Fedora**: `password-auth` and `postlogin`. authselect puts `pam_fprintd`
  in `system-auth` and `fingerprint-auth`, never in `password-auth`.
- **NixOS**: `security.pam.services.lys-login.startSession = true`, and
  `fprintAuth = false` (see [Fingerprint](#fingerprint)).

Elsewhere, adapt one of the packaged files.

## Autologin

`--autologin USER` logs USER in once per boot, before the first greeter, through
`lys-login-autologin`: `pam_systemd_loadkey` then `pam_permit`, with the account,
session and password stacks of the display manager. If it fails the greeter
shows. After USER logs out the greeter shows.

The worker then reads the `cryptsetup` key that systemd-cryptsetup left in the
kernel keyring at boot and hands it to the [keyring](#the-keyring) like a typed
password. The unit runs with `KeyringMode=inherit` to see that key. When the
disk passphrase is also the keyring password, one prompt at boot unlocks the disk,
logs in and opens the keyring.

With no key there (no LUKS, TPM unlock, initramfs-tools' cryptroot) or a
passphrase the keyring rejects, the worker logs it and the keyring stays locked
until the lock screen opens it. `login.lock.lock-on-start` locks the session as
it starts.

## Fingerprint

Under the display manager the greeter takes a password or a fingerprint,
whichever comes first. As soon as a user is known (remembered, picked, or typed
and confirmed with Enter) the greeter starts the login, and the session worker
asks fprintd over the system bus to verify that user's finger while the greeter
waits for the password. While the reader listens, a fingerprint prompt stands in
for the password field, as on the [lock screen](/login/guide/lock-screen#fingerprint).

- **Password first**: the worker stops the scan and answers the first prompt of
  `lys-login` with the typed password. A wrong
  password is refused at once; the greeter starts listening again.
- **Fingerprint first**: the worker withdraws the password prompt, so no
  `lys-login` transaction starts, and logs the user in through
  `lys-login-fingerprint`:
  `pam_nologin`, `pam_systemd_loadkey`, then `pam_permit`, with the account,
  session and password stacks of `lys-login`. Only the worker, as root and after
  fprintd reported a match, opens that service.
- **No match**: the greeter says so and keeps listening, and the password still
  logs in.

`pam_fprintd` must not be in `lys-login`: it blocks the password until it times
out. The NixOS module turns it off there (`services.fprintd.enable` adds it to
every service). The Debian and Fedora services keep it out by not including
`common-auth` or `system-auth`.

`login.greeter.fingerprint: false` in the greeter's config turns the
fingerprint off: the worker never asks fprintd, and the password alone logs in,
which also opens the [keyring](#the-keyring). The lock screen keeps its own
`login.lock.fingerprint`.

No password is typed, so the [keyring](#the-keyring) gets the `cryptsetup` key
from the kernel keyring, as [autologin](#autologin) does. Without one the keyring
stays locked until the lock screen opens it, which the worker logs.

Without fprintd, a reader or an enrolled finger, and while logind reports the lid
closed, the password alone logs in. Under greetd, greetd's own service decides.

## The lock screen's service

The lock screen authenticates against `login.lock.pam-service`, default
`system-auth`. That service exists on Arch and Fedora but not on Debian or NixOS.

- **Debian, Ubuntu, Fedora**: the package installs `lys-shell`: `pam_unix`
  over `common-account` on Debian, `password-auth` on Fedora, so pam_fprintd
  never sits in it and a typed password is checked at once while the lock
  screen scans for a fingerprint itself. Point the config at it:

  ```yaml
  login:
    lock:
      pam-service: lys-shell
  ```

  Fedora's `system-auth` carries pam_fprintd when authselect's
  `with-fingerprint` is on, which makes a typed password wait for the scan.
- **NixOS**: `programs.lys-login.lock.enable = true` declares the service named
  by `lock.pamService` (default `lys-shell`). Set the same name in the config.
  Its gnome-keyring lines are enabled unless `services.lys-secrets.enable` is set.
- **Elsewhere** (home-manager's `lock.enable` only prints these steps): name an
  existing service or create `/etc/pam.d/lys-shell`:

  ```text
  auth     required   pam_unix.so
  auth     optional   pam_gnome_keyring.so
  account  required   pam_unix.so
  password required   pam_unix.so
  password optional   pam_gnome_keyring.so use_authtok
  session  required   pam_unix.so
  session  optional   pam_gnome_keyring.so auto_start
  ```

  `pam_unix` must be `required`: `sufficient` skips the keyring lines. The
  keyring lines are `optional` and do nothing without gnome-keyring (Debian
  package `libpam-gnome-keyring`).

## The keyring

No PAM module is needed to open the keyring. After a successful authentication
the password goes to the login collection of lys-secrets, else gnome-keyring,
starting either on the session bus if needed. When PAM changes an expired
password, the lys-secrets collection is re-keyed to the new one. Failures are
logged (`journalctl -u lys-login -p warning` under the daemon) and never block the
login or unlock.

KWallet opens only through `pam_kwallet` in the PAM stack.
[Architecture](/login/reference/architecture#the-keyring) has the details.

## Test a service

[pamtester](https://pamtester.sourceforge.net/) runs a service without logging out:

```sh
pamtester lys-shell "$USER" authenticate
```
