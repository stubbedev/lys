{
  name = "accounts-daemon renames alice through SetRealName";
  module = {
    services.accounts-daemon.enable = true;
    security.polkit.enable = true;
    users.users.alice = {
      isNormalUser = true;
      description = "Alice";
    };
  };
  script = ''
    machine.wait_for_unit("multi-user.target")
    path = machine.succeed("busctl --system call org.freedesktop.Accounts /org/freedesktop/Accounts org.freedesktop.Accounts FindUserById x 1000 | cut -d\\\" -f2").strip()
    machine.succeed(f"busctl --system call org.freedesktop.Accounts {path} org.freedesktop.Accounts.User SetRealName s 'Alice Liddell'")
    gecos = machine.succeed("getent passwd alice | cut -d: -f5").strip()
    assert gecos.split(",")[0] == "Alice Liddell", gecos
  '';
}
