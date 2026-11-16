{
  pkgs,
  lib,
  self,
}:

let
  pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.lys-shell;
  probe = pkg.overrideAttrs (_: {
    pname = "lys-shell-clockwatch-test";
    goTests = [ "shell/internal/clockwatch" ];
    buildPhase = ''
      runHook preBuild
      go test -c -o clockwatch.test ./shell/internal/clockwatch
      runHook postBuild
    '';
    installPhase = ''
      install -Dm0755 clockwatch.test $out/bin/clockwatch.test
    '';
    postInstall = "";
    postFixup = "";
    doCheck = false;
  });
in
{
  name = "internal/clockwatch sees a step of the clock and a suspend as jumps";
  module = {
    virtualisation = {
      useNixStoreImage = true;
      mountHostNixStore = false;
      sharedDirectories = lib.mkForce { };
    };
  };
  script = ''
    machine.wait_for_unit("multi-user.target")
    probe = "${probe}/bin/clockwatch.test -test.v -test.count=1"
    machine.succeed(f"LYS_CLOCKWATCH_VM=1 {probe} -test.run 'TestAStepOfTheClockIsAJump' >&2", timeout=120)
    machine.succeed(f"LYS_CLOCKWATCH_VM=1 {probe} -test.run 'TestAResumeIsAJump' >&2", timeout=180)
  '';
}
