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
      pkgs,
      ...
    }: let
      cfg = config.programs.lys-settings;
      declared = pkgs.writeText "lys-users-nixos.json" (builtins.toJSON {
        inherit (config.users) mutableUsers;
        users = lib.attrNames config.users.users;
      });
    in {
      imports = [options];
      config = lib.mkIf cfg.enable {
        environment.systemPackages = [cfg.package];
        services.dbus.packages = [cfg.package];
        systemd.packages = [cfg.package];
        systemd.services.lys-users.serviceConfig.ExecStart = ["" "${cfg.package}/libexec/lys-users --nixos ${declared}"];
        security.polkit.enable = lib.mkDefault true;
        services.userdbd.enable = true;
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
