{
  perSystem = {
    pkgs,
    self',
    ...
  }: {
    devShells.power = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-power];

      packages = with pkgs; [
        gopls
        dbus
        golangci-lint
        delve
        just
        git
        gh
      ];

      env = {
        CGO_ENABLED = 0;
        GOTOOLCHAIN = "local";
      };
    };
  };
}
