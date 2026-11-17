resource "laravelcloud_environments" "example" {
  application = "application-id"
  name        = "production"
  branch      = "main"
}
