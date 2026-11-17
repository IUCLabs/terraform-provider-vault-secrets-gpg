data "gpg_key" "app" {
  backend = "gpg"
  name    = "app"
}

output "app_fingerprint" {
  value = data.gpg_key.app.fingerprint
}
