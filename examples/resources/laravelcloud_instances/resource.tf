resource "laravelcloud_instances" "example" {
  environment        = "environment-id"
  min_replicas       = 1
  name               = "example-name"
  polling_interval   = 1
  scaling_type       = "none"
  shutdown_timeout   = 1
  size               = "flex-512mb"
  type               = "service"
  visibility_timeout = 1
}
