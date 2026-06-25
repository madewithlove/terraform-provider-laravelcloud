resource "laravelcloud_websocket_clusters" "example" {
  max_connections = 1
  name            = "example-name"
  region          = "us-east-2"
  type            = "reverb"
}
