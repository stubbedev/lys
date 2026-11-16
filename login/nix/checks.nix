{
  self,
  ...
}: {
  perSystem = {
    lib,
    pkgs,
    ...
  }: let
    pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-login;
    fakeLysWm = pkgs.writeShellScriptBin "lys-wm" "";
    greeterSystemWith = extra: greeter:
      (pkgs.nixos {
        imports = [self.nixosModules.lys-login extra];
        programs.lys-login.greeter = {enable = true;} // greeter;
        boot.loader.grub.enable = false;
        fileSystems."/" = {
          device = "nodev";
          fsType = "tmpfs";
        };
        system.stateVersion = lib.trivial.release;
      }).config;
    greeterSystem = greeterSystemWith {};
    daemonSystem = greeter:
      greeterSystem ({
          displayManager = "lys-login";
          compositorPackage = fakeLysWm;
        }
        // greeter);
    failedAssertions = system: map (a: a.message) (lib.filter (a: !a.assertion) system.assertions);
  in {
    checks =
      lib.optionalAttrs pkgs.stdenv.hostPlatform.isx86_64 (let
        vm = import ./tests/vm.nix {inherit pkgs lib;};
        args = {inherit pkgs lib self vm;};
        greeter = import ./tests/greeter.nix args;
      in {
        vms-greeter = greeter "greeter" ["login" "accessibility" "x11_session" "keyboard_german" "keyboard_switching" "keyring" "no_keyring"];
        vms-sessions = greeter "sessions" ["lock" "rekey" "switch" "upgrade"];
        vms-greetd = import ./tests/greetd.nix args;
        vms-autologin = import ./tests/autologin.nix args;
      })
      // {
        upgrade-unit = let
          unit = (daemonSystem {session.environment = ["A=b c"];}).systemd.services.lys-login;
          daemonArgs = lib.last (lib.splitString " daemon " unit.serviceConfig.ExecStart);
        in
          pkgs.runCommand "lys-login-upgrade-unit" {} ''
            ${lib.optionalString (!unit.reloadIfChanged) ''echo "a switch does not reload the changed unit" >&2; exit 1''}
            ${lib.optionalString unit.restartIfChanged ''echo "a switch may restart the display manager under a session" >&2; exit 1''}
            grep -qF -- ${lib.escapeShellArg " upgrade -- ${daemonArgs} "} ${unit.serviceConfig.ExecReload} || { echo "ExecReload upgrades to other arguments than ExecStart runs with" >&2; exit 1; }
            touch "$out"
          '';
        splash-unit = let
          splash = pkgs.writeShellScriptBin "lys-splash" "";
          unit = splashPackage: (daemonSystem {inherit splashPackage;}).systemd.services.lys-login;
          handsOver = u:
            lib.elem "lys-splash-quit.service" u.conflicts
            && lib.all (s: lib.elem s u.after) ["lys-splash-start.service" "lys-splash-quit.service"]
            && (u.serviceConfig.ExecStartPre or null) == "-${lib.getExe splash} deactivate"
            && lib.hasInfix (builtins.unsafeDiscardStringContext "--splash ${lib.getExe splash} ") (builtins.unsafeDiscardStringContext u.serviceConfig.ExecStart);
          touchesSplash = u:
            lib.any (s: lib.hasInfix "lys-splash" (builtins.unsafeDiscardStringContext s)) (u.conflicts ++ u.after ++ [u.serviceConfig.ExecStart (u.serviceConfig.ExecStartPre or "")]);
        in
          pkgs.runCommand "lys-login-splash-unit" {} ''
            ${lib.optionalString (!handsOver (unit splash)) ''echo "with splashPackage the unit does not take the screen from lys-splash" >&2; exit 1''}
            ${lib.optionalString (touchesSplash (unit null)) ''echo "without splashPackage the unit names lys-splash" >&2; exit 1''}
            touch "$out"
          '';
        greeter-sessions-assertion = let
          rejects = session: lib.any (lib.hasInfix "offers no session to log into") (failedAssertions (daemonSystem {inherit session;}));
        in
          pkgs.runCommand "lys-login-greeter-sessions-assertion" {} ''
            ${lib.optionalString (!rejects {
              dirs = [];
              x11Dirs = [];
            }) ''echo "no session dirs and no session.command passed evaluation" >&2; exit 1''}
            ${lib.optionalString (rejects {}) ''echo "the default session dirs were rejected" >&2; exit 1''}
            ${lib.optionalString (rejects {
              dirs = [];
              x11Dirs = [];
              command = "niri --session";
            }) ''echo "session.command alone was rejected" >&2; exit 1''}
            ${lib.optionalString (rejects {
              dirs = [];
              x11Dirs = ["/x"];
            }) ''echo "x11Dirs alone was rejected" >&2; exit 1''}
            touch "$out"
          '';
        greetd-software-render = let
          command = renderer:
            (greeterSystem {
              displayManager = "greetd";
              inherit renderer;
            }).services.greetd.settings.default_session.command;
          has = needle: renderer: lib.hasInfix needle (builtins.unsafeDiscardStringContext (command renderer));
        in
          pkgs.runCommand "lys-login-greetd-software-render" {} ''
            ${lib.optionalString (!(has "LYS_UI_RENDERER=cpu" "software") || !(has "WLR_RENDERER=pixman" "software")) ''echo "renderer = software leaves cage or the greeter on the GPU" >&2; exit 1''}
            ${lib.optionalString (has "LYS_UI_RENDERER" "auto" || has "WLR_RENDERER" "auto") ''echo "renderer = auto forces a software path" >&2; exit 1''}
            touch "$out"
          '';
        daemon-software-render = let
          envLysWm = pkgs.writeShellScriptBin "lys-wm" ''echo "LYS_WM_RENDERER=''${LYS_WM_RENDERER-} LYS_UI_RENDERER=''${LYS_UI_RENDERER-}"'';
          system = renderer:
            daemonSystem {
              inherit renderer;
              compositorPackage = envLysWm;
            };
          compositor = renderer: lib.head (lib.splitString " " (lib.last (lib.splitString " --compositor " (system renderer).systemd.services.lys-login.serviceConfig.ExecStart)));
          rejects = greeter: needle: lib.any (lib.hasInfix needle) (failedAssertions (daemonSystem greeter));
        in
          pkgs.runCommand "lys-login-daemon-software-render" {} ''
            ${lib.optionalString (failedAssertions (system "software") != []) ''echo "renderer = software failed evaluation under the lys-login display manager" >&2; exit 1''}
            [ "$(${compositor "software"})" = "LYS_WM_RENDERER=gles LYS_UI_RENDERER=cpu" ] || { echo "renderer = software leaves the daemon's lys-wm on Vulkan or the greeter on the GPU" >&2; exit 1; }
            [ "$(${compositor "auto"})" = "LYS_WM_RENDERER= LYS_UI_RENDERER=" ] || { echo "renderer = auto forces a software path on the daemon's lys-wm" >&2; exit 1; }
            ${lib.optionalString (!rejects {graphicsWrapper = ["nixGL"];} "greeter.graphicsWrapper") ''echo "a graphicsWrapper passed evaluation under the lys-login display manager" >&2; exit 1''}
            ${lib.optionalString (rejects {} "greeter.graphicsWrapper") ''echo "the default graphicsWrapper was rejected" >&2; exit 1''}
            touch "$out"
          '';
        fingerprint-pam = let
          system =
            greeterSystemWith {
              services.fprintd.enable = true;
              programs.lys-login.lock.enable = true;
            } {
              displayManager = "lys-login";
              compositorPackage = fakeLysWm;
            };
          pamText = service: system.security.pam.services.${service}.text;
          fingerprint = pamText "lys-login-fingerprint";
        in
          pkgs.runCommand "lys-login-fingerprint-pam" {} ''
            ${lib.optionalString (!lib.hasInfix "pam_fprintd" (pamText "login")) ''echo "services.fprintd.enable puts no pam_fprintd in the login stack, so this check proves nothing" >&2; exit 1''}
            ${lib.optionalString (lib.hasInfix "pam_fprintd" (pamText "lys-login")) ''echo "the lys-login password stack waits on pam_fprintd before the password" >&2; exit 1''}
            ${lib.optionalString (lib.hasInfix "pam_fprintd" (pamText system.programs.lys-login.lock.pamService)) ''echo "the lock screen's password stack waits on pam_fprintd, though the lock screen scans for a fingerprint itself" >&2; exit 1''}
            ${lib.optionalString (lib.hasInfix "pam_fprintd" fingerprint || lib.hasInfix "pam_unix" fingerprint) ''echo "lys-login-fingerprint authenticates again after the daemon verified the fingerprint" >&2; exit 1''}
            ${lib.optionalString (!lib.hasInfix "pam_permit.so" fingerprint || !lib.hasInfix "pam_nologin.so" fingerprint || !lib.hasInfix "pam_systemd_loadkey.so" fingerprint) ''echo "lys-login-fingerprint is not nologin, the disk key, then permit" >&2; exit 1''}
            ${lib.optionalString (!lib.hasInfix "account  include   lys-login" fingerprint || !lib.hasInfix "session  include   lys-login" fingerprint) ''echo "lys-login-fingerprint skips the account or session stack of lys-login" >&2; exit 1''}
            touch "$out"
          '';
        default-display-manager = let
          system = greeterSystem {};
          greetd = greeterSystem {displayManager = "greetd";};
          runsDaemonIn = compositor: sys:
            lib.hasInfix
            (builtins.unsafeDiscardStringContext " daemon --vt 1 --service lys-login --greeter-user lys-greeter --compositor ${lib.getExe' compositor "lys-wm"} ")
            (builtins.unsafeDiscardStringContext sys.systemd.services.lys-login.serviceConfig.ExecStart);
          defaultCompositor = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-wm;
        in
          pkgs.runCommand "lys-login-default-display-manager" {} ''
            ${lib.optionalString (failedAssertions system != []) ''echo "greeter.enable alone fails an assertion" >&2; exit 1''}
            ${lib.optionalString (!(system.systemd.services ? lys-login) || !(runsDaemonIn defaultCompositor system)) ''echo "greeter.enable alone does not run the lys-login daemon in the flake's lys-wm" >&2; exit 1''}
            ${lib.optionalString system.services.greetd.enable ''echo "greeter.enable alone enabled greetd" >&2; exit 1''}
            ${lib.optionalString (!(runsDaemonIn fakeLysWm (daemonSystem {}))) ''echo "compositorPackage does not reach the daemon" >&2; exit 1''}
            ${lib.optionalString (!greetd.services.greetd.enable || greetd.systemd.services ? lys-login) ''echo "displayManager = greetd does not run greetd instead of the daemon" >&2; exit 1''}
            ${lib.optionalString (!system.security.polkit.enable) ''echo "the lys-login daemon runs without polkit, which authorizes apply-config" >&2; exit 1''}
            touch "$out"
          '';
        # The package wraps the binary with the libraries it dlopens, libpam
        # above all.
        package = pkgs.runCommand "lys-login-package" {} ''
          grep -q "${lib.getLib pkgs.pam}/lib" ${pkg}/bin/lys-login || { echo "lys-login lacks libpam on its library path" >&2; exit 1; }
          grep -q "${pkgs.xkeyboard-config}/share/X11/xkb" ${pkg}/bin/.lys-login-wrapped || { echo "lys-login lacks the keyboard data path" >&2; exit 1; }
          grep -q "${pkgs.libX11}/share/X11/locale" ${pkg}/bin/.lys-login-wrapped || { echo "lys-login lacks the locale data path" >&2; exit 1; }
          grep -q '<action id="dev.stubbe.lys.login.apply-config">' ${pkg}/share/polkit-1/actions/dev.stubbe.lys.login.policy || { echo "lys-login does not install its polkit action" >&2; exit 1; }
          touch "$out"
        '';
      };
  };
}
