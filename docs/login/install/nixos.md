# NixOS

With the lys flake as an input ([NixOS](/install/nixos)), import
`lys.nixosModules.lys-login`.

## Display manager

```nix
programs.lys-login.greeter = {
  enable = true;
  settings = {
    styling.appearance = "dark";
    login.background-mode = "color";
    login.background-color = "#1e1e2e";
  };
};
```

This runs `lys-login daemon` as `display-manager.service` with the flake's
lys-wm, and adds the `lys-login`, `lys-login-autologin` and
`lys-login-fingerprint` PAM services (with `pam_fprintd` off in `lys-login`), the `lys-greeter` user and `/var/lib/lys-greeter`. `settings` becomes
`/etc/lys-shell/config.yaml` ([Configuration](/login/guide/configuration)). Sessions
come from the NixOS sessions directory, which collects every installed
compositor's session file.

`nixos-rebuild switch` never restarts the unit. When it changed, the switch
reloads it, which [upgrades the daemon in place](/login/reference/architecture#upgrading-in-place).
If the upgrade fails the old daemon keeps running, the switch exits 4 and lists
`lys-login-upgrade-failed.service`, whose status has the reason.

To use the lys-wm your desktop runs:

```nix
programs.lys-login.greeter.compositorPackage = config.programs.lys-wm.package;
```

lys-wm renders with Vulkan and stops on a card without a Vulkan driver, such as
a VM's virtio-gpu without venus. There, `greeter.renderer = "software"` runs the
greeter's lys-wm with Vulkan on lavapipe, Mesa's CPU driver; set
`environment.sessionVariables.LYS_WM_RENDERER = "lavapipe"` instead to cover the
greeter and every lys-wm session, which inherit it through PAM. lavapipe shares
buffers through `/dev/udmabuf`, which systemd grants the seat's sessions.

With [lys-splash](/splash/) as boot splash, set
`greeter.splashPackage`. The unit then conflicts with `lys-splash-quit.service`,
which `multi-user.target` no longer wants, so a switch leaves the session alone,
runs `lys-splash deactivate` before lys-wm starts, and the daemon runs
`lys-splash quit --retain-splash` once the greeter's first frame is presented,
so there is no blank frame: the greeter's first frame is the splash's last one,
the logo where the splash left it, and the login screen fades in over it
([Taking over from the splash](/login/reference/architecture#taking-over-from-the-splash)).

## Lock screen

```nix
programs.lys-login = {
  enable = true;
  lock.enable = true;
};
```

`enable` (or `greeter.enable`) puts `lys-login` on the PATH. `lock.enable` adds
the [PAM service](/login/guide/pam#the-lock-screen-s-service). Set
`login.lock.pam-service` in your lys-shell config to `lock.pamService`.

## Under greetd

`greeter.displayManager = "greetd"` runs the greeter under greetd in the same
lys-wm kiosk the daemon uses, started by `lys-login kiosk`, with state owned by
greetd's `greeter` user.

## Options

All under `programs.lys-login`.

| Option | Type | Default | Effect |
| --- | --- | --- | --- |
| `enable` | bool | `false` | `lys-login` on the PATH. |
| `package` | package | the flake's lys-login | The lys-login package. |
| `autologin.user` | null or string | `null` | Account logged in once per boot before the first greeter ([Autologin](/login/guide/pam#autologin)). Requires `displayManager = "lys-login"`. |
| `autologin.session` | null or string | `null` | Session id to start (a session file's name, `-x11` suffix for X11); null takes the last session, else the first. |
| `lock.enable` | bool | `false` | Declare the lock screen's PAM service. |
| `lock.pamService` | string | `"lys-shell"` | Its name. |
| `greeter.enable` | bool | `false` | Run the greeter via `greeter.displayManager`. |
| `greeter.displayManager` | `"lys-login"` or `"greetd"` | `"lys-login"` | `lys-login`: the daemon on `greeter.vt`. `greetd`: greetd running `lys-login kiosk`. Both show the greeter in the lys-wm kiosk. |
| `greeter.compositorPackage` | package | the flake's lys-wm | lys-wm for the greeter kiosk, under either display manager. |
| `greeter.splashPackage` | null or package | `null` | lys-splash, when it is the boot splash. |
| `greeter.vt` | positive int | `1` | VT the daemon runs on. |
| `greeter.settings` | YAML | `{}` | Written to `/etc/lys-shell/config.yaml` when non-empty. |
| `greeter.session.dirs` | list of strings | NixOS `wayland-sessions` | Directories of Wayland session files. |
| `greeter.session.x11Dirs` | list of strings | NixOS `xsessions` | X11 session files, started through `startx` (needs it on the PATH, e.g. `services.xserver.enable`). `[]` disables X11. |
| `greeter.session.command` | string | `""` | Fallback session offered as "Custom"; the only entry when no session files exist. |
| `greeter.session.environment` | list of strings | `[]` | Extra `KEY=value` for every session. |
| `greeter.renderer` | `"auto"` or `"software"` | `"auto"` | `software` needs no GPU driver: the greeter draws on the CPU, lys-wm renders with Vulkan on lavapipe (`LYS_WM_RENDERER=lavapipe`). Sessions are not affected. |
| `greeter.graphicsWrapper` | list of strings | `[]` | greetd only: command prefix around the launch, e.g. nixGL. |

Evaluation fails when no session can be offered, when `autologin.user` is set
under greetd, or when `graphicsWrapper` is set under the daemon.
