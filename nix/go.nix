{lib, ...}: let
  components = ["lib" "ui" "desktop" "wm" "shell" "login" "settings" "polkit" "secrets" "splash"];
  docsReadByTests = map (path: ../. + "/${path}") (
    lib.filter (lib.hasPrefix "docs/") (
      lib.concatMap (c: c.paths or []) (lib.attrValues (lib.importJSON ../ci/components.json))
    )
  );
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.difference (lib.fileset.gitTracked ../.) (
      lib.fileset.unions (
        [
          ../flake.nix
          ../flake.lock
          ../nix
          (lib.fileset.difference ../docs (lib.fileset.unions docsReadByTests))
          (lib.fileset.maybeMissing ../.github)
        ]
        ++ map (c: lib.fileset.maybeMissing (../. + "/${c}/nix")) components
      )
    );
  };
  version = "0-unstable-${lib.substring 0 8 (builtins.unsafeDiscardStringContext (baseNameOf src))}";
in {
  perSystem = {
    inputs',
    pkgs,
    ...
  }: let
    go = inputs'.devtools.packages.go;
    buildGoModule = pkgs.buildGoModule.override {inherit go;};
  in {
    _module.args = {
      inherit go buildGoModule;
      testFontsConf = pkgs.makeFontsConf {
        fontDirectories = [pkgs.dejavu_fonts pkgs.noto-fonts-color-emoji];
      };
      lysGo = {
        inherit src version;
        build = args:
          buildGoModule ({
              inherit src version;
              vendorHash = "sha256-Ns+RMSC1saI7/iMLw/knGryqFRsywzT0zDE8/eXpRoA=";
              overrideModAttrs = _: {name = "lys-go-modules";};
              env.CGO_ENABLED = "0";
            }
            // args
            // {
              ldflags = (args.ldflags or []) ++ ["-X github.com/stubbedev/lys/lib/buildinfo.version=${version}"];
            });
        test = packages: ''
          runHook preCheck
          go test ${lib.escapeShellArgs packages}
          runHook postCheck
        '';
      };
    };
  };
}
