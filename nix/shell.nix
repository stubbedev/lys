{
  perSystem = {
    pkgs,
    self',
    ...
  }: {
    devShells.default = pkgs.mkShell {
      inputsFrom = map (c: self'.devShells.${c}) ["lib" "ui" "desktop" "wm" "shell" "login" "settings" "polkit" "power" "secrets" "splash"];
    };
    formatter = pkgs.alejandra;
  };
}
