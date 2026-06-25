resource "laravelcloud_caches" "example" {
  auto_upgrade_enabled = false
  is_public            = false
  name                 = "example-name"
  region               = "us-east-2"
  size                 = "250mb"
  type                 = "laravel_valkey"
}
