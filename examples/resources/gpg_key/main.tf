resource "gpg_key" "app" {
  backend    = "gpg"
  name       = "app"
  algorithm  = "rsa-4096" # rsa-2048 | rsa-3072 | rsa-4096 | ed25519
  identity   = "App <app@unkin.net>"
  exportable = false
}

output "app_public_key" {
  value = gpg_key.app.public_key
}

output "app_fingerprint" {
  value = gpg_key.app.fingerprint
}
