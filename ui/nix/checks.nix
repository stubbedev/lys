{
  perSystem = {self', ...}: {
    checks = {
      inherit (self'.packages) lys-ui;
    };
  };
}
