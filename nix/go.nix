{lib, ...}: let
  root = ../.;
  inRoot = rel: root + "/${rel}";
  components = ["lib" "ui" "desktop" "wm" "shell" "login" "settings" "viewer" "files" "polkit" "power" "secrets" "splash"];
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

  closure = {
    name,
    packages,
    tests,
  }: let
    roots = lib.concatMap (expandPattern name) packages;
  in
    if roots == [] && tests == []
    then throw "lysGo: ${name} builds no Go package; set subPackages, goPackages or goTests"
    else importClosure name (roots ++ tests ++ lib.concatMap (dir: (goPackage name dir).testImports or []) tests);

  source = {
    name,
    built,
    tests,
    resources,
  }:
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

  isStandard = path: !lib.hasInfix "." (lib.head (lib.splitString "/" path));
  entryImports = entry: (entry.external or []) ++ (entry.testExternal or []);
  moduleImports = lib.sort lib.lessThan (lib.unique (
    lib.filter (path: !isStandard path) (lib.concatMap entryImports (lib.attrValues goPackages))
  ));
  externalModuleImports = {
    name,
    built,
    tests,
  }:
    lib.sort lib.lessThan (lib.unique (lib.filter (path: !isStandard path) (
      lib.concatMap (dir: (goPackage name dir).external or []) built
      ++ lib.concatMap (dir: (goPackage name dir).testExternal or []) tests
    )));
in {
  perSystem = {
    inputs',
    pkgs,
    system,
    ...
  }: let
    go = inputs'.devtools.packages.go;
    buildGoModule = pkgs.buildGoModule.override {inherit go;};
    distGo = let
      release = {
        version = "1.27.4";
        x86_64-linux = {
          platform = "linux-amd64";
          hash = "sha256-ciZzbCM5cZzaKPvzpU1xqmO87+N26YzvAANCaNqySEA=";
        };
        aarch64-linux = {
          platform = "linux-arm64";
          hash = "sha256-M5JYKYiic6h6tQQ+4bLJwXosPABYC/kybEei4KqurJI=";
        };
      };
      inherit (release.${system}) platform hash;
    in
      if release.version != go.version
      then throw "dist-go: nix/go.nix pins go.dev's Go ${release.version}, but the dev shells' Go is ${go.version}; set its version and the hashes go.dev/dl/?mode=json lists for go${go.version}.linux-amd64.tar.gz and go${go.version}.linux-arm64.tar.gz"
      else
        pkgs.runCommandLocal "go-${release.version}-${platform}" {
          src = pkgs.fetchurl {
            url = "https://go.dev/dl/go${release.version}.${platform}.tar.gz";
            inherit hash;
          };
        } ''
          mkdir "$out"
          tar -xzf "$src" -C "$out" --strip-components=1
        '';
    goEnv = {
      proxyVendor = true;
      env.CGO_ENABLED = "0";
    };
    modules =
      goEnv
      // {
        vendorHash = "sha256-O6m3T0jDMKnON0A9MiRDJG8eoEuE3iwqIxNHiIK0l/A=";
        overrideModAttrs = _: {
          name = "lys-go-modules";
          src = modulesSource;
        };
      };
    seedBuildCache = layers:
      ''
        mkdir -p "$GOCACHE"
      ''
      + lib.concatMapStrings (layer: ''
        cp -rs --no-preserve=mode ${layer}/. "$GOCACHE"
      '')
      layers
      + ''
        date +%s > "$GOCACHE/trim.txt"
      '';
    buildCacheLayer = {
      name,
      version,
      config,
      packages,
      seeds,
    }:
      (buildGoModule (config
        // {
          pname = "lys-go-build-cache-${name}";
          inherit version;
          importPaths = lib.concatLines packages;
          passAsFile = ["importPaths"];
          postConfigure = seedBuildCache seeds;
          buildPhase = ''
            runHook preBuild
            mapfile -t buildable < <(xargs go list -e -f '{{if and (not .Error) (not .DepsErrors) .GoFiles}}{{.ImportPath}}{{end}}' < "$importPathsPath")
            go build -p "$NIX_BUILD_CORES" "''${buildable[@]}"
            runHook postBuild
          '';
          installPhase = ''
            runHook preInstall
            rm "$GOCACHE/trim.txt"
            find "$GOCACHE" -type f -name '*-a' -exec sed -i -E 's/.{20}$/                   0/' {} +
            mkdir "$out"
            (cd "$GOCACHE" && find . -type f -exec cp --parents -t "$out" {} +)
            runHook postInstall
          '';
          doCheck = false;
          dontFixup = true;
        }))
      .overrideAttrs {disallowedReferences = [];};
    buildCaches = {
      env,
      modulePackages,
    }: let
      extra = lib.removeAttrs env (lib.attrNames goEnv.env);
      suffix = lib.concatMapStrings (name: "-${lib.toLower extra.${name}}") (lib.attrNames extra);
      std = buildCacheLayer {
        name = "std${suffix}";
        inherit (go) version;
        config =
          goEnv
          // {
            inherit env;
            vendorHash = null;
            src = pkgs.writeTextDir "go.mod" ''
              module lys-go-build-cache-std

              go ${go.version}
            '';
          };
        packages = ["std"];
        seeds = [];
      };
    in {
      inherit std;
      modules = buildCacheLayer {
        name = "modules${suffix}";
        version = lib.substring 0 8 (builtins.hashString "sha256" (lib.concatLines modulePackages));
        config = modules // {inherit env; src = modulesSource;};
        packages = modulePackages;
        seeds = [std];
      };
    };
    sharedCaches = buildCaches {
      inherit (goEnv) env;
      modulePackages = moduleImports;
    };
  in {
    packages.dist-go = distGo;
    _module.args = {
      inherit go buildGoModule;
      testFontsConf = pkgs.makeFontsConf {
        fontDirectories = [pkgs.dejavu_fonts pkgs.noto-fonts-color-emoji];
      };
      lysGo = {
        inherit src;
        build = args:
          if args ? postConfigure
          then throw "lysGo: ${args.pname} sets postConfigure, which lysGo.build uses to seed the Go build cache"
          else
            buildGoModule (finalAttrs: let
              targets = {
                name = finalAttrs.pname;
                packages = (finalAttrs.subPackages or []) ++ finalAttrs.goPackages;
                tests = finalAttrs.goTests;
              };
              built = closure targets;
              env = goEnv.env // (args.env or {});
              externals = externalModuleImports {
                inherit built;
                inherit (targets) name tests;
              };
              caches =
                if env == goEnv.env
                then sharedCaches
                else
                  buildCaches {
                    inherit env;
                    modulePackages = externals;
                  };
            in
              modules
              // {
                goPackages = [];
                goTests = [];
                resources = [];
              }
              // args
              // {
                src = source {
                  inherit built;
                  inherit (targets) name tests;
                  inherit (finalAttrs) resources;
                };
                inherit env;
                postConfigure = seedBuildCache ([caches.std] ++ lib.optional (externals != []) caches.modules);
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
