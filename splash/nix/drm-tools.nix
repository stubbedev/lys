{
  lib,
  runCommandCC,
  pkg-config,
  libdrm,
}: let
  tools = {
    drm-grab = ./drm-grab.c;
    scanout-watch = ./scanout-watch.c;
  };
in
  runCommandCC "lys-splash-drm-tools" {
    nativeBuildInputs = [pkg-config];
    buildInputs = [libdrm];
  } ''
    mkdir -p $out/bin
    ${lib.concatStrings (lib.mapAttrsToList (name: source: ''
        $CC -std=gnu11 -O2 -Wall -Wextra -Werror -o $out/bin/${name} ${source} $(pkg-config --cflags --libs libdrm)
      '')
      tools)}
  ''
