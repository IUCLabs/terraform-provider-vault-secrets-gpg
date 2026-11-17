resource "gpg_key" "app" {
  backend   = "gpg"
  name      = "app"
  algorithm = "rsa-4096"
  identity  = "App <app@example.com>"
}

output "app_public_key" {
  value = gpg_key.app.public_key
}
