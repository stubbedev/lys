{
  pkgs,
  lys-wm,
  lys-ui,
}:
lys-wm.overrideAttrs (old: {
  pname = "lys-wm-headless";
  doCheck = true;
  nativeCheckInputs = import ./test-programs.nix pkgs;
  env =
    old.env
    // import ./test-env.nix pkgs
    // {LYS_UI_TEST_CLIENTS = "${import ./lys-ui-debug.nix lys-ui}/bin";};
  checkPhase = ''
    runHook preCheck
    export HOME="$TMPDIR"
    just test
    runHook postCheck
  '';
})
