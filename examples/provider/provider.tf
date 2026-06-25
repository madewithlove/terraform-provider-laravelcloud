terraform {
  required_providers {
    laravelcloud = {
      source = "madewithlove/laravelcloud"
    }
  }
}

provider "laravelcloud" {
  # The API token can also be supplied via the LARAVEL_CLOUD_TOKEN environment
  # variable, which is the recommended approach for secrets.
  api_token = var.laravel_cloud_token
}

variable "laravel_cloud_token" {
  type      = string
  sensitive = true
}
