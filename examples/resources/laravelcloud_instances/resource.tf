# A worker instance. Every environment also has an "app" instance, created with
# the environment; import it to manage it (see import.sh).
resource "laravelcloud_instances" "example" {
  environment    = "environment-id"
  name           = "example-name"
  type           = "service"
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
  uses_scheduler = false
}
