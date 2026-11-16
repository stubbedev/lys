pkgs: {
  name,
  source,
  libraries,
  modules,
}:
pkgs.runCommandCC name {
  nativeBuildInputs = [pkgs.pkg-config];
  buildInputs = libraries;
} ''
  read -ra flags <<< "$(pkg-config --cflags --libs ${toString modules})"
  mkdir -p "$out/bin"
  "$CC" -std=gnu11 -O2 -Wall -Wextra -Werror -o "$out/bin/${name}" ${source} "''${flags[@]}"
''
