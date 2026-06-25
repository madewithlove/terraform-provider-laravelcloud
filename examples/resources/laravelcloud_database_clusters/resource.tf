resource "laravelcloud_database_clusters" "example" {
  config = {}
  name   = "example-name"
  region = "us-east-2"
  type   = "laravel_mysql_84"
}
