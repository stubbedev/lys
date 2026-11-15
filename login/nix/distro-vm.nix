# What `just login::distro-vm <debian|fedora>` boots: the pinned cloud images,
# and the guest tree its distro-vm-guest recipe builds the release's .deb or
# .rpm from, laid out as the repository is. lys-secrets' package is built
# beside it from the flake's static lys-secrets, so the login can unlock its
# login collection.
{
  lib,
  self,
  ...
}: {
  perSystem = {pkgs, ...}: let
    images = {
      debian = pkgs.fetchurl {
        url = "https://cloud.debian.org/images/cloud/trixie/20261026-2643/debian-13-generic-amd64-20261026-2643.qcow2";
        sha512 = "5bd891c6e62d01639f52a5587b72a4607421e445732214be787dba6043cf9797a6ff90bed982aada6a6c7e619f1f956d961c4fad76757171abd607920aac098a";
      };
      fedora = pkgs.fetchurl {
        url = "https://dl.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2";
        sha256 = "28680fe5b371a5a82ebf43a31926e086a168e59949d03969c5093e7071f90b7f";
      };
    };
    lysSecrets = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-secrets;
    tree = lib.fileset.toSource {
      root = ../..;
      fileset = lib.fileset.unions [
        ../../packaging/justfile
        ../justfile
        ../packaging
        ../scripts/distro-vm
        ../../secrets/justfile
        ../../secrets/LICENSE
        ../../secrets/packaging
        ../../secrets/resources
      ];
    };
    guest = pkgs.runCommand "lys-login-distro-vm-guest" {} ''
      cp -r ${tree} $out
      chmod -R u+w $out
      install -Dm755 ${lysSecrets}/bin/lys-secrets $out/secrets/dist/lys-secrets
    '';
  in
    lib.mkIf pkgs.stdenv.hostPlatform.isx86_64 {
      packages =
        {login-distro-vm-guest = guest;}
        // lib.mapAttrs' (distro: image: lib.nameValuePair "login-distro-vm-image-${distro}" image) images;

      # The guest's files parse: the justfile its recipes are in, the session
      # script and the greeter.
      checks.distro-vm-scripts = pkgs.runCommand "lys-login-distro-vm-scripts" {nativeBuildInputs = [pkgs.just pkgs.shellcheck pkgs.python3];} ''
        just --justfile ${guest}/login/justfile --summary >/dev/null
        just --justfile ${guest}/packaging/justfile --summary >/dev/null
        shellcheck ${guest}/login/scripts/distro-vm/session.sh
        python3 -c 'import ast, sys; ast.parse(open(sys.argv[1]).read())' ${guest}/login/scripts/distro-vm/greeter.py
        touch $out
      '';
    };
}
