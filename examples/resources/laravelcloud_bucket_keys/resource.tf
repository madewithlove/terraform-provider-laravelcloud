resource "laravelcloud_bucket_keys" "example" {
  filesystem = "filesystem-id"
  name       = "example-name"
  permission = "read_write"
}
