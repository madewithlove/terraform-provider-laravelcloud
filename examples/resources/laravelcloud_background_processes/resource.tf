resource "laravelcloud_background_processes" "example" {
  instance  = "instance-id"
  type      = "worker"
  processes = 1
  config = {
    connection = "database"
    queue      = "default"
    tries      = 1
    timeout    = 300
  }
}
