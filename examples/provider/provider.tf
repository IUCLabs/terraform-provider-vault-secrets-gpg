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
  backend  = gpg_secret_backend.gpg.path
  name     = "app"
  identity = "App <app@example.com>"
}
