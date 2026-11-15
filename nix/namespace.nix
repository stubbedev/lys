lib: component: path: let
  name = check:
    if check == "lys-${component}"
    then "${component}-package"
    else "${component}-${lib.removePrefix "${component}-" (lib.removePrefix "lys-${component}-" check)}";
  rename = checks: lib.mapAttrs' (check: lib.nameValuePair (name check)) checks;
  inPerSystem = perSystem:
    if lib.isFunction perSystem
    then lib.setFunctionArgs (args: inResult (perSystem args)) (lib.functionArgs perSystem)
    else inResult perSystem;
  inResult = result: result // lib.optionalAttrs (result ? checks) {checks = rename result.checks;};
  inModule = module: module // lib.optionalAttrs (module ? perSystem) {perSystem = inPerSystem module.perSystem;};
  module = import path;
in
  if lib.isFunction module
  then lib.setFunctionArgs (args: inModule (module args)) (lib.functionArgs module)
  else inModule module
