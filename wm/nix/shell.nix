{
  inputs,
  self,
  ...
}: {
  perSystem = {
    inputs',
    pkgs,
    self',
    ...
  }: {
    devShells.wm = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-wm];

      packages = with pkgs;
        [
          gopls
          golangci-lint
          delve
          govulncheck
          just
          git
          gh
        ]
        ++ ["${inputs.lys-ci}"]
        ++ import ./test-programs.nix {
          inherit pkgs;
          lys-ui = self'.packages.lys-ui.overrideAttrs {doCheck = false;};
        };

      env =
        import ./test-env.nix pkgs
        // {
          CGO_ENABLED = "0";
          GOTOOLCHAIN = "local";
        };
    };
  };
}
