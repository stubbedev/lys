{
  lib,
  pkgs,
}: {
  package,
  renderer,
}:
if renderer == "software"
then
  pkgs.runCommand "lys-wm-software" {nativeBuildInputs = [pkgs.makeBinaryWrapper];} ''
    makeBinaryWrapper ${lib.getExe' package "lys-wm"} $out/bin/lys-wm ${
      lib.concatStringsSep " " (lib.mapAttrsToList (k: v: "--set ${k} ${lib.escapeShellArg v}") (import ./software-render-env.nix).lys-wm)
    }
  ''
else package
