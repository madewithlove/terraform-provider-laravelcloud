resource "laravelcloud_databases" "example" {
  config = {}
  name   = "example-name"
  region = "us-east-2"
  type   = "laravel_mysql_8"
}
