resource "laravelcloud_filesystem_keys" "example" {
  filesystem = "filesystem-id"
  name       = "example-name"
  permission = "read_write"
}
