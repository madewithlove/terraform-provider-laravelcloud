resource "laravelcloud_applications" "example" {
  name       = "example-name"
  region     = "us-east-2"
  repository = "example-org/example-repository"
}
