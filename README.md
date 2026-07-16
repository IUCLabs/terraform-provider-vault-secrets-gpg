# terraform-provider-gpgvaultsecret

A Terraform/OpenTofu provider for the
[vault-plugin-secrets-gpg](https://git.unkin.net/unkin/vault-plugin-secrets-gpg)
engine. It manages the engine **mount** (and, optionally, plugin catalog
registration) and **OpenPGP keys** on HashiCorp Vault or OpenBao.

The provider's source address is `git.unkin.net/unkin/gpgvaultsecret`; its
resources are prefixed `gpg_` (declare it under the local name `gpg`).

## Resources & data sources

| Type | Purpose |
|------|---------|
| `gpg_secret_backend` (resource) | Mount the gpg engine at a path. When `sha256` is set, register the plugin in the catalog first. |
| `gpg_key` (resource) | Create and configure an OpenPGP key (algorithm, identity, exportable, deletion, min decryption version); exposes the armored public key, fingerprint, key id and latest version. |
| `gpg_key` (data source) | Read an existing key's metadata + armored public key. |

## Usage

```hcl
terraform {
  required_providers {
    gpg = {
      source = "artifactapi.k8s.syd1.au.unkin.net/terraform-unkin/gpgvaultsecret"
    }
  }
}

provider "gpg" {
  # address and token fall back to VAULT_ADDR / VAULT_TOKEN
  address = "https://vault.main.unkin.net"
}

# Mount the engine. Omit sha256 if the plugin is already registered out of band
# (e.g. by terraform-vault). Set it to have this resource register the plugin.
resource "gpg_secret_backend" "gpg" {
  path        = "gpg"
  description = "GPG/OpenPGP secrets engine"
  # sha256    = "0e92d7408795688badb55789bc1604e8f1dd4d71998656c7f831991fce9a7b20"
}

resource "gpg_key" "app" {
  backend   = gpg_secret_backend.gpg.path
  name      = "app"
  algorithm = "rsa-4096" # rsa-2048 | rsa-3072 | rsa-4096 | ed25519
  identity  = "App <app@unkin.net>"
}

# Wire the armored public key somewhere (e.g. a pass store, a k8s secret).
output "app_public_key" {
  value = gpg_key.app.public_key
}
```

### `gpg_secret_backend`

| Attribute | | Description |
|-----------|-----|-------------|
| `path` | required, force-new | Mount path (e.g. `gpg`). |
| `plugin` | optional, force-new | Registered plugin name/mount type. Default `vault-plugin-secrets-gpg`. |
| `description` | optional | Mount description. |
| `sha256` | optional | If set, (re)register the plugin in the catalog with this binary hash before mounting. |
| `command` | optional | Binary filename in the server `plugin_directory` used for registration. Defaults to `plugin`. |

Import: `terraform import gpg_secret_backend.gpg gpg`.

### `gpg_key`

| Attribute | | Description |
|-----------|-----|-------------|
| `backend` | required, force-new | Mount path of the engine. |
| `name` | required, force-new | Key name. |
| `algorithm` | optional, force-new | `rsa-2048` / `rsa-3072` (default) / `rsa-4096` / `ed25519`. |
| `identity` | optional, force-new | OpenPGP User ID. Defaults to the key name. |
| `exportable` | optional | Allow private-key export (enable-only). |
| `deletion_allowed` | optional | Whether the key may be deleted. `terraform destroy` enables this automatically. |
| `min_decryption_version` | optional | Minimum version usable for decrypt/verify. |
| `latest_version`, `fingerprint`, `key_id`, `public_key` | computed | Current key material metadata. |

Import: `terraform import gpg_key.app gpg/app`.

## Build

```sh
make build     # -> terraform-provider-gpgvaultsecret
make install   # into ~/.terraform.d/plugins/... for local use
make test      # go test -race (unit tests; no Vault needed)
make e2e       # real terraform apply/destroy against a Vault dev server + the gpg plugin
make package   # zip for the artifactapi terraform registry
```

CI (Woodpecker) runs pre-commit/build/lint/test on PRs. On a `v*` tag it builds
and PUTs `terraform-provider-gpgvaultsecret_<version>_linux_amd64.zip` to the
artifactapi `terraform-unkin` registry (GPG-signed server-side), installable via
the bare `source = "artifactapi.k8s.syd1.au.unkin.net/terraform-unkin/gpgvaultsecret"`.
