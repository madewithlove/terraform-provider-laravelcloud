resource "laravelcloud_filesystems" "example" {
  jurisdiction   = "default"
  key_name       = "example-key_name"
  key_permission = "read_write"
  name           = "example-name"
  visibility     = "private"
}
