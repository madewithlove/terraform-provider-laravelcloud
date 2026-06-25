resource "laravelcloud_background_processes" "example" {
  processes = 1
  type      = "worker"
}
