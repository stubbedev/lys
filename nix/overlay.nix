{self, ...}: {
  flake.overlays.default = _final: prev:
    builtins.mapAttrs (name: _: self.packages.${prev.stdenv.hostPlatform.system}.${name}) {
      lys-wm = null;
      lys-shell = null;
      lys-login = null;
      lys-settings = null;
      lys-viewer = null;
      lys-polkit = null;
      lys-power = null;
      lys-secrets = null;
      lys-splash = null;
    };
}
