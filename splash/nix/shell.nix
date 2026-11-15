{
  inputs,
  self,
  ...
}: {
  perSystem = {
    pkgs,
    self',
    ...
  }: {
    devShells.splash = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-splash];

      packages = with pkgs; [
        gopls
        golangci-lint
        delve
        just
        git
        gh
        "${inputs.lys-ci}"
      ];

      env = {
        CGO_ENABLED = 0;
        GOTOOLCHAIN = "local";
      };
    };

  };
}
