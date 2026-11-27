{pkgs, ...}: {
  services.udev.packages = [
    (pkgs.writeTextDir "lib/udev/rules.d/10-lys-login-vm-late-udmabuf-grant.rules" ''
      ACTION=="change", KERNEL=="udmabuf", RUN+="${pkgs.coreutils}/bin/sleep 0.5"
    '')
  ];
}
