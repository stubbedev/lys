self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.lys-power;
in {
  options.services.lys-power = {
    enable = lib.mkEnableOption "lys-power, adaptive battery charging and power profile switching";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-power;
      defaultText = lib.literalExpression "lys.packages.\${system}.lys-power";
      description = "The lys-power package, whose system unit and tmpfiles rule are installed.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.upower.enable = lib.mkDefault true;
    services.power-profiles-daemon.enable = lib.mkDefault true;

    environment.systemPackages = [cfg.package];
    systemd.packages = [cfg.package];
    systemd.tmpfiles.packages = [cfg.package];
    systemd.services.lys-power.wantedBy = ["graphical.target"];
  };
}
