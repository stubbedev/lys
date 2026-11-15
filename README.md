<p align="center">
  <img src="docs/public/lys.svg" width="160" alt="lys">
</p>

<h1 align="center">lys</h1>

<p align="center">
  A Wayland desktop in pure Go: compositor, shell, display manager, settings
  and the session services around them.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/">Manual</a>
</p>

## Components

| Directory | What it is | Binary or import path |
| --- | --- | --- |
| [`shell/`](shell) | Desktop shell: bar, notifications, launcher, wallpaper, dialogs, desktop portal | `lys-shell` |
| [`wm/`](wm) | Tiling Wayland compositor | `lys-wm`, `lys-wmctl` |
| [`login/`](login) | Display manager, greeter and lock screen | `lys-login` |
| [`settings/`](settings) | Settings app for lys-shell and lys-wm | `lys-settings` |
| [`polkit/`](polkit) | Polkit authentication agent | `lys-polkit` |
| [`secrets/`](secrets) | Secret Service and Bitwarden vault | `lys-secrets` |
| [`splash/`](splash) | Boot splash, in place of Plymouth | `lys-splash` |
| [`lib/`](lib) | Rendering, Vulkan, Wayland, media, PipeWire, D-Bus and PAM | `github.com/stubbedev/lys/lib/...` |
| [`ui/`](ui) | Widget toolkit | `github.com/stubbedev/lys/ui/...` |
| [`desktop/`](desktop) | Config, theming, translations and service clients the apps share | `github.com/stubbedev/lys/desktop/...` |

Everything builds with `CGO_ENABLED=0`. Libraries such as libvulkan, libva and
libpam are loaded at run time where used.

## Install

The [manual's install section](https://stubbedev.github.io/lys/install/)
covers NixOS, Debian and Ubuntu, Fedora and other systems. In short:

- **NixOS:** add `inputs.lys.url = "github:stubbedev/lys";` and import
  `lys.nixosModules.lys-<app>` for each app.
- **Debian, Ubuntu, Fedora:** install the `.deb` or `.rpm` files from the
  [latest release](https://github.com/stubbedev/lys/releases/latest).
- **From source:** `go install github.com/stubbedev/lys/<app>/cmd/lys-<app>@latest`.

Every app is released together: one `vX.Y.Z` tag, one release page carrying
every app's tarball, `.deb` and `.rpm`.

## Use as a library

```sh
go get github.com/stubbedev/lys
```

```go
import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)
```

[lys-ui](https://stubbedev.github.io/lys/ui/) and
[lys-lib](https://stubbedev.github.io/lys/lib/) have guides; the API reference
is on [pkg.go.dev](https://pkg.go.dev/github.com/stubbedev/lys).

## Repository layout

One Go module, `github.com/stubbedev/lys`, at the root. Each component
directory holds its packages, its `cmd/` binaries, its packaging and its own
`README.md`. `docs/` is the manual, one VitePress site with a section per
component, published to <https://stubbedev.github.io/lys/>.

## License

MIT. Each component directory has its `LICENSE` and the notices of what it
embeds.
