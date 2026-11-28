self: {
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.lys-files;
in {
  options.programs.lys-files = {
    enable = lib.mkEnableOption "lys-files, the lys desktop's file manager, with the udev rule that lets the seat's user open phones and cameras";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-files;
      defaultText = lib.literalExpression "lys.packages.\${system}.lys-files";
      description = "The lys-files package, whose udev rule tags phones and cameras for the seat's user.";
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [cfg.package];
    services.udev.packages = [cfg.package];
  };
}
