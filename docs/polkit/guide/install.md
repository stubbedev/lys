# Install

Disable any other polkit agent the session starts; polkitd accepts one per
session.

## Home-manager

For NixOS or any distribution with nix:

```nix
{
  inputs.lys.url = "github:stubbedev/lys";

  # in the home-manager configuration
  imports = [inputs.lys.homeManagerModules.lys-polkit];
  services.lys-polkit.enable = true;
  services.hyprpolkitagent.enable = false; # one agent per session
}
```

This installs the package and the `lys-polkit.service` user unit, wanted by
`graphical-session.target`. On NixOS, polkitd comes from
`security.polkit.enable = true`.

## Without nix

Requires the Go version declared in `go.mod`.

```sh
git clone https://github.com/stubbedev/lys
cd lys
go build -ldflags "-X main.version=$(git describe --always)" -o lys-polkit ./polkit/cmd/lys-polkit
install -Dm755 lys-polkit ~/.local/bin/lys-polkit
mkdir -p ~/.config/systemd/user
sed "s|@bindir@|$HOME/.local/bin|" polkit/dist/lys-polkit.service.in > ~/.config/systemd/user/lys-polkit.service
systemctl --user daemon-reload
systemctl --user enable --now lys-polkit.service
```

`getent` must be on the service's `PATH` (see [identities](./how-it-works#identities)).

Then [check that it works](./getting-started#check).
