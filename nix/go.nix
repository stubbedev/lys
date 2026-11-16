{lib, ...}: let
  root = ../.;
  inRoot = rel: root + "/${rel}";
  components = ["lib" "ui" "desktop" "wm" "shell" "login" "settings" "polkit" "power" "secrets" "splash"];
  tracked = lib.fileset.gitTracked root;
  docsReadByTests = map inRoot (
    lib.filter (lib.hasPrefix "docs/") (
      lib.concatMap (c: c.paths or []) (lib.attrValues (lib.importJSON ../ci/components.json))
    )
  );
  src = lib.fileset.toSource {
    inherit root;
    fileset = lib.fileset.difference tracked (
      lib.fileset.unions (
        [
          ../flake.nix
          ../flake.lock
          ../nix
          (lib.fileset.difference ../docs (lib.fileset.unions docsReadByTests))
          (lib.fileset.maybeMissing ../.github)
        ]
        ++ map (c: lib.fileset.maybeMissing (inRoot "${c}/nix")) components
      )
    );
  };

  modulesSource = lib.fileset.toSource {
    inherit root;
    fileset = lib.fileset.unions [../go.mod ../go.sum];
  };

  goPackages = lib.importJSON ./go-packages.json;
  goPackageDirs = lib.attrNames goPackages;
  goPackage = consumer: dir:
    goPackages.${dir} or (throw "lysGo: ${consumer} names the Go package ${dir}, which nix/go-packages.json does not list; run just go-packages");
  expandPattern = consumer: pattern: let
    prefix = lib.removeSuffix "/..." pattern;
    matches = lib.filter (dir: dir == prefix || lib.hasPrefix "${prefix}/" dir) goPackageDirs;
  in
    if prefix == pattern
    then [pattern]
    else if matches == []
    then throw "lysGo: ${consumer} names ${pattern}, which matches no Go package in nix/go-packages.json"
    else matches;
  importClosure = consumer: roots:
    map (entry: entry.key) (builtins.genericClosure {
      startSet = map (key: {inherit key;}) roots;
      operator = {key}: map (key: {inherit key;}) ((goPackage consumer key).imports or []);
    });
  goFiles = withTests: dir: let
    entries = builtins.readDir (inRoot dir);
    wanted = name:
      entries.${name}
      == "regular"
      && !lib.hasPrefix "." name
      && !lib.hasPrefix "_" name
      && (lib.hasSuffix ".s" name || lib.hasSuffix ".go" name && (withTests || !lib.hasSuffix "_test.go" name));
  in
    map (name: inRoot "${dir}/${name}") (lib.filter wanted (lib.attrNames entries));
  existing = consumer: rel:
    if builtins.pathExists (inRoot rel)
    then inRoot rel
    else throw "lysGo: ${consumer} names the source ${rel}, which does not exist";

  source = {
    name,
    packages,
    tests,
    resources,
  }: let
    roots = lib.concatMap (expandPattern name) packages;
    built = importClosure name (roots ++ tests ++ lib.concatMap (dir: (goPackage name dir).testImports or []) tests);
  in
    if roots == [] && tests == []
    then throw "lysGo: ${name} builds no Go package; set subPackages, goPackages or goTests"
    else
      lib.fileset.toSource {
        inherit root;
        fileset = lib.fileset.intersection tracked (lib.fileset.unions (
          [../go.mod ../go.sum]
          ++ lib.concatMap (goFiles false) built
          ++ lib.concatMap (goFiles true) tests
          ++ map inRoot (lib.concatMap (dir: (goPackage name dir).embeds or []) built)
          ++ map inRoot (lib.concatMap (dir: (goPackage name dir).testEmbeds or []) tests)
          ++ map (existing name) resources
        ));
      };
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
        inherit src;
        build = args:
          buildGoModule (finalAttrs:
            {
              vendorHash = "sha256-qfi2oHcO6KggNDRIi9Ni3whcR2KZvhpKwb18TW1Hb0c=";
              proxyVendor = true;
              overrideModAttrs = _: {
                name = "lys-go-modules";
                src = modulesSource;
              };
              env.CGO_ENABLED = "0";
              goPackages = [];
              goTests = [];
              resources = [];
            }
            // args
            // {
              src = source {
                name = finalAttrs.pname;
                packages = (finalAttrs.subPackages or []) ++ finalAttrs.goPackages;
                tests = finalAttrs.goTests;
                inherit (finalAttrs) resources;
              };
              version = "0-unstable-${lib.substring 0 8 (builtins.unsafeDiscardStringContext (baseNameOf finalAttrs.src))}";
              ldflags = (args.ldflags or []) ++ ["-X github.com/stubbedev/lys/lib/buildinfo.version=${finalAttrs.version}"];
            });
        test = args: ''
          runHook preCheck
          go test ${lib.escapeShellArgs args}
          runHook postCheck
        '';
      };
    };
  };
}
