resource "laravelcloud_instances" "example" {
  environment  = "environment-id"
  max_replicas = 1
  min_replicas = 1
  name         = "example-name"
  scaling_type = "none"
  size         = "flex.c-1vcpu-256mb"
  type         = "service"
}
