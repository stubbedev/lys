{
  perSystem = {
    lib,
    lysGo,
    ...
  }: {
    packages.lys-secrets = lysGo.build {
      pname = "lys-secrets";

      subPackages = ["secrets/cmd/lys-secrets"];

      ldflags = ["-s" "-w"];

      # The suite runs in `just check`: it needs private D-Bus sessions and
      # the Secret Service clients of the dev shell.
      doCheck = false;

      # The user unit and the D-Bus activation files, for the modules to
      # wire up. Installing the package alone activates neither, since
      # gnome-keyring's activation file may claim the same name.
      # pinentry-lys runs the binary as GnuPG's pinentry, for a
      # pinentry-program line, which takes no arguments.
      postInstall = ''
        ln -s lys-secrets $out/bin/pinentry-lys
        install -d $out/lib/systemd/user $out/share/dbus-1/services
        substitute secrets/resources/lys-secrets.service $out/lib/systemd/user/lys-secrets.service \
          --replace-fail /usr/bin/lys-secrets "$out/bin/lys-secrets"
        for activation in org.freedesktop.secrets dev.stubbe.lys.Vault1; do
          substitute secrets/resources/$activation.service $out/share/dbus-1/services/$activation.service \
            --replace-fail /usr/bin/lys-secrets "$out/bin/lys-secrets"
        done
      '';

      meta = {
        description = "the lys desktop's freedesktop Secret Service";
        homepage = "https://github.com/stubbedev/lys/tree/main/secrets";
        license = lib.licenses.mit;
        mainProgram = "lys-secrets";
        platforms = lib.platforms.linux;
      };
    };
  };
}
