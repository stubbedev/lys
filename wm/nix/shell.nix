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
        ++ import ./test-programs.nix pkgs;

      env =
        import ./test-env.nix pkgs
        // {
          CGO_ENABLED = "0";
          GOTOOLCHAIN = "local";
          LYS_DIST_GO = "${self'.packages.dist-go}/bin/go";
        };
    };
  };
}
