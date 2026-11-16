{
  perSystem = {
    lib,
    lysGo,
    pkgs,
    ...
  }: let
    kernelCommandLineConditions = lib.concatMapStringsSep "\n" (c: "ConditionKernelCommandLine=${c}") [
      "!plymouth.enable=0"
      "!nosplash"
      "!splash=verbose"
      "|splash"
      "|rhgb"
    ];
    shutdownHook = "lys-splash-shutdown-hook";
  in {
    packages.lys-splash = lysGo.build {
      pname = "lys-splash";
      subPackages = ["splash/cmd/lys-splash"];

      doCheck = false;

      outputs = ["out" "compat"];

      dontPatchShebangs = true;
      dontMoveSbin = true;

      postInstall = ''
        installUnits() {
          mkdir -p "$1"
          for unit in splash/units/*; do
            substitute "$unit" "$1/$(basename "$unit")" \
              --subst-var-by lys_splash "$2" \
              --subst-var-by kernel_command_line_conditions ${lib.escapeShellArg kernelCommandLineConditions} \
              --subst-var-by systemd_tty_ask_password_agent "$3"
          done
        }
        installShutdownHook() {
          mkdir -p "$1/lib/systemd/system-shutdown"
          ln -s ../../../bin/lys-splash "$1/lib/systemd/system-shutdown/${shutdownHook}"
        }
        installUnits $out/lib/systemd/system $out/bin/lys-splash ${lib.getExe' pkgs.systemd "systemd-tty-ask-password-agent"}
        installShutdownHook $out

        install -Dm755 $out/bin/lys-splash $compat/bin/lys-splash
        ln -s lys-splash $compat/bin/plymouth
        mkdir -p $compat/sbin $compat/share
        ln -s ../bin/lys-splash $compat/sbin/plymouthd
        installShutdownHook $compat
        installUnits $compat/lib/systemd/system /usr/bin/lys-splash /usr/bin/systemd-tty-ask-password-agent
        cp -r splash/initramfs-tools $compat/share/initramfs-tools

        if grep -rn '@[a-z_]*@' $out/lib $compat/lib; then
          echo "a unit names a placeholder that is not filled in" >&2
          exit 1
        fi
      '';

      passthru = {inherit shutdownHook;};

      meta = {
        description = "The lys desktop's boot splash";
        homepage = "https://github.com/stubbedev/lys/tree/main/splash";
        license = lib.licenses.mit;
        mainProgram = "lys-splash";
        platforms = lib.platforms.unix;
      };
    };
  };
}
