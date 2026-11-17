# Terraform Provider for vault-plugin-secrets-gpg

This provider manages the
[vault-plugin-secrets-gpg](https://github.com/IUCLabs/vault-plugin-secrets-gpg)
secrets engine on HashiCorp Vault or OpenBao, with Terraform or OpenTofu. Use
it to mount the engine (and, optionally, register the plugin in the catalog),
and to create OpenPGP keys whose private material stays inside Vault.

The registry type is `vault-secrets-gpg`; its resources are prefixed `gpg_`,
so declare the provider under the local name `gpg`.

## Quick Start

Prebuilt provider archives are on the [releases page](https://github.com/IUCLabs/terraform-provider-vault-secrets-gpg/releases).

```hcl
terraform {
  required_providers {
    gpg = {
      source = "IUCLabs/vault-secrets-gpg"
    }
  }
}

# address and token fall back to VAULT_ADDR and VAULT_TOKEN.
provider "gpg" {
  address = "https://vault.example.com:8200"
}

resource "gpg_secret_backend" "gpg" {
  path = "gpg"
}

resource "gpg_key" "app" {
  backend   = gpg_secret_backend.gpg.path
  name      = "app"
  algorithm = "rsa-4096"
  identity  = "App <app@example.com>"
}

output "app_public_key" {
  value = gpg_key.app.public_key
}
```

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0+ or [OpenTofu](https://opentofu.org/)
- HashiCorp Vault or OpenBao, with the
  [vault-plugin-secrets-gpg](https://github.com/IUCLabs/vault-plugin-secrets-gpg)
  binary in the server's `plugin_directory` (see that repo for the server
  versions it is tested against)
- [Go](https://go.dev/doc/install) 1.25+ (only if you build from source)

## Usage

See the [docs](docs/index.md) for the provider, `gpg_secret_backend`,
`gpg_key` and the `gpg_key` data source.

## Development

```sh
make build   # build the provider binary
make test    # unit tests
make docs    # regenerate docs/ from the schema and examples/
make e2e     # apply real Terraform against a Vault dev server
```

## License

[MIT](LICENSE)
