pkgs: {
  name = "cursor";
  outputs = ["Virtual-1" "Virtual-2"];
  kernelModules.vkms = "enable_cursor=0";
  framedCursors = ["Virtual-2"];
  machineConfig.environment.systemPackages = [pkgs.drm_info];
  scripts = [./cursor.py];
}
