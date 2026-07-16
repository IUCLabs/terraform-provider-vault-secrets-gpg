data "gpg_key" "app" {
  backend = "gpg"
  name    = "app"
}

# e.g. import into a pass store or hand to another system
output "app_public_key" {
  value = data.gpg_key.app.public_key
}
