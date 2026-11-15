<p align="center">
  <img src="../docs/public/lys-secrets.svg" width="160" alt="lys-secrets">
</p>

<h1 align="center">lys-secrets</h1>

<p align="center">
  The freedesktop
  <a href="https://specifications.freedesktop.org/secret-service-spec/latest/">Secret Service</a>
  and Bitwarden vault of <a href="../README.md">lys</a>.
</p>

<p align="center">
  <a href="https://stubbedev.github.io/lys/secrets/">Documentation</a>
</p>

lys-secrets serves `org.freedesktop.secrets` for libsecret (`secret-tool`),
Chromium and Electron apps, `git-credential-libsecret`, python-secretstorage and
the Secret portal, with collections encrypted at rest. It also serves your
Bitwarden vault (`dev.stubbe.lys.Vault1`) to lys-shell, and can act as SSH agent
and GnuPG pinentry. Password prompts use the desktop's dialogs.

Any program running as you can read an unlocked collection; see
[How secrets are protected](https://stubbedev.github.io/lys/secrets/guide/security).

## Install

Disable any other Secret Service provider first.

- **NixOS / home-manager:** import `lys.nixosModules.lys-secrets` or
  `lys.homeModules.lys-secrets` from `github:stubbedev/lys` and set
  `services.lys-secrets.enable = true;`.
- **Debian, Ubuntu, Fedora:** install the `.deb` or `.rpm` from the
  [latest release](https://github.com/stubbedev/lys/releases/latest), then
  `systemctl --user enable --now lys-secrets`.
- **Others:** see
  [Getting started](https://stubbedev.github.io/lys/secrets/guide/getting-started).

## Usage

Log in through lys-login, or unlock at its lock screen, to unlock the `login`
collection. Then:

```sh
secret-tool store --label='An example' app example
secret-tool lookup app example
```
