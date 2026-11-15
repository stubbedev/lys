{
  perSystem = {
    pkgs,
    self',
    ...
  }: {
    devShells.polkit = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-polkit];

      packages = with pkgs; [
        gopls
        golangci-lint
        delve
        just
        git
        gh
        dbus
        getent
      ];

      env = {
        CGO_ENABLED = 0;
        GOTOOLCHAIN = "local";
      };
    };

  };
}
