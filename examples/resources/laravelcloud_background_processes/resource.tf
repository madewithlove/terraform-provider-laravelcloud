resource "laravelcloud_background_processes" "example" {
  instance  = "instance-id"
  processes = 1
  type      = "worker"
}
