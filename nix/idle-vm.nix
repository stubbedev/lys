{
  pkgs,
  self,
}: let
  user = "alice";
  shellConfig = clock:
    builtins.toJSON {
      animations.indicators = false;
      wallpaper.wallpaper = "${flatWallpaper}";
      modules = {inherit clock;};
      bar.layout = [
        {
          monitor = "*";
          left = ["workspaces"];
          center = ["clock"];
          right = ["battery"];
        }
      ];
    };
  flatWallpaper = pkgs.runCommand "flat-wallpaper.png" {nativeBuildInputs = [pkgs.imagemagick];} ''
    magick -size 64x64 xc:'#20242c' "png:$out"
  '';
  ticking = pkgs.writeText "lys-shell-ticking.yaml" (shellConfig {format = "%H:%M:%S";});
in {
  name = "idle";
  outputs = ["Virtual-1" "Virtual-2"];
  backgroundOnly = false;

  home.".config/lys-shell/config.yaml" = shellConfig {format = "%H:%M";};

  machineConfig = {
    imports = [
      self.nixosModules.lys-shell
      self.nixosModules.lys-power
    ];
    programs.lys-shell = {
      enable = true;
      systemd.enable = true;
    };
    services.lys-power.enable = true;
    networking.hosts."127.0.0.1" = ["api.open-meteo.com"];
    environment.sessionVariables.LIBGL_ALWAYS_SOFTWARE = "1";
    hardware.graphics.extraPackages = [pkgs.mesa];
    fonts.packages = [pkgs.inter];
    environment.systemPackages = [pkgs.weston];
    users.users.${user}.extraGroups = ["video" "render" "input"];
  };

  params = {
    ticking = "${ticking}";
    budgets = builtins.fromJSON (builtins.readFile ./idle-budgets.json);
  };

  scripts = [./idle-vm.py];
}
