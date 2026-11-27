{
  lib,
  lysGo,
}: let
  env = lib.importJSON ../dochelper/goenv.json;
in
  lysGo.build {
    pname = "lys-docthumb";

    subPackages = ["desktop/cmd/lys-docthumb"];

    inherit env;

    ldflags = ["-s" "-w"];

    doCheck = false;

    doInstallCheck = true;
    installCheckPhase = ''
      runHook preInstallCheck
      go version -m $out/bin/lys-docthumb > buildinfo
      ${lib.concatMapStrings (name: ''
        if ! grep -qxF ${lib.escapeShellArg "\tbuild\t${name}=${env.${name}}"} buildinfo; then
          echo "lys-docthumb is not built with ${name}=${env.${name}}:" >&2
          cat buildinfo >&2
          exit 1
        fi
      '') (lib.attrNames env)}
      runHook postInstallCheck
    '';

    meta = {
      description = "The lys desktop's document helper, which renders documents in a process of its own";
      homepage = "https://github.com/stubbedev/lys/tree/main/desktop";
      license = lib.licenses.mit;
      mainProgram = "lys-docthumb";
      platforms = lib.platforms.linux;
    };
  }
