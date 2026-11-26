{
  inputs,
  self,
  ...
}: {
  perSystem = {
    lib,
    pkgs,
    self',
    ...
  }: let
    gpuDrivers = self.lib.gpuDriverEnv pkgs;
  in {
    # lys-desktop's dev shell: the lint stack, fonts and the headless sway
    # and private D-Bus the tests start, with its fontconfig and GPU
    # driver setup (its shellHook).
    devShells.login = pkgs.mkShell {
      inputsFrom = [self'.packages.lys-login self'.devShells.desktop];

      packages = ["${inputs.lys-ci}"];

      LD_LIBRARY_PATH = lib.makeLibraryPath (lib.unique (gpuDrivers.libraries ++ self'.packages.lys-login.libs));
      CGO_ENABLED = "0";
      GOTOOLCHAIN = "local";
      LYS_DIST_GO = "${self'.packages.dist-go}/bin/go";
    };

  };
}
