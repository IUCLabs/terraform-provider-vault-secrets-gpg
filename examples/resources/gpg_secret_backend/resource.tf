# Mount a plugin that is already registered in the catalog.
resource "gpg_secret_backend" "gpg" {
  path        = "gpg"
  description = "OpenPGP keys"
}

# Register the binary in the server's plugin_directory, then mount it.
variable "plugin_sha256" {
  type = string
}

resource "gpg_secret_backend" "registered" {
  path    = "gpg-registered"
  sha256  = var.plugin_sha256
  command = "vault-plugin-secrets-gpg"
}
