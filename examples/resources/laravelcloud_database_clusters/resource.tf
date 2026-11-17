resource "laravelcloud_database_clusters" "example" {
  name   = "example-name"
  region = "us-east-2"
  # The API reports the engine without its version ("laravel_mysql"). A
  # versioned type such as "laravel_mysql_84" is kept as configured.
  type = "laravel_mysql"

  # Settings depend on the type. Values are strings; booleans and numbers are
  # sent to the API as such.
  config = {
    size                     = "mysql-flex-512mb"
    storage                  = "5"
    is_public                = "false"
    uses_scheduled_snapshots = "true"
    retention_days           = "7"
  }
}
