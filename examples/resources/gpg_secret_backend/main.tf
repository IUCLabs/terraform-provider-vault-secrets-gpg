resource "gpg_secret_backend" "gpg" {
  path        = "gpg"
  description = "GPG/OpenPGP secrets engine"

  # Optional: have this resource register the plugin in the catalog. Omit if the
  # plugin is already registered (e.g. by terraform-vault).
  # sha256  = "0e92d7408795688badb55789bc1604e8f1dd4d71998656c7f831991fce9a7b20"
  # command = "vault-plugin-secrets-gpg"
}
