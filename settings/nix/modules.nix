{self, ...}: let
  options = {
    lib,
    pkgs,
    ...
  }: {
    options.programs.lys-settings = {
      enable = lib.mkEnableOption "lys-settings, the lys desktop's settings app";
      package = lib.mkOption {
        type = lib.types.package;
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-settings;
        defaultText = lib.literalExpression "lys.packages.\${system}.lys-settings";
        description = "The lys-settings package to install.";
      };
    };
  };
in {
  flake = {
    nixosModules.lys-settings = {
      config,
      lib,
      ...
    }: {
      imports = [options];
      config = lib.mkIf config.programs.lys-settings.enable {
        environment.systemPackages = [config.programs.lys-settings.package];
        security.polkit.enable = lib.mkDefault true;
      };
    };

    homeModules.lys-settings = {
      config,
      lib,
      ...
    }: {
      imports = [options];
      config = lib.mkIf config.programs.lys-settings.enable {
        home.packages = [config.programs.lys-settings.package];
      };
    };
  };
}
