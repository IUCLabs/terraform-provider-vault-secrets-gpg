terraform {
  required_providers {
    gpg = {
      source = "git.unkin.net/unkin/gpgvaultsecret"
    }
  }
}

variable "address" { type = string }
variable "token" { type = string }
variable "plugin_sha256" { type = string }

provider "gpg" {
  address = var.address
  token   = var.token
}

# Register the plugin in the catalog (sha256) and mount the engine at gpg/.
resource "gpg_secret_backend" "gpg" {
  path        = "gpg"
  sha256      = var.plugin_sha256
  description = "GPG/OpenPGP secrets engine"
}

# A managed key.
resource "gpg_key" "app" {
  backend    = gpg_secret_backend.gpg.path
  name       = "app"
  algorithm  = "rsa-2048"
  identity   = "App <app@unkin.net>"
  exportable = false
}

# Read it back via the data source.
data "gpg_key" "app" {
  backend    = gpg_secret_backend.gpg.path
  name       = gpg_key.app.name
  depends_on = [gpg_key.app]
}

output "resource_fingerprint" { value = gpg_key.app.fingerprint }
output "resource_public_key" { value = gpg_key.app.public_key }
output "data_public_key" { value = data.gpg_key.app.public_key }
output "latest_version" { value = gpg_key.app.latest_version }
