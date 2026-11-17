variable "website_token" {
  type      = string
  sensitive = true
}

resource "laravelcloud_environment_variable" "website_token" {
  environment = "environment-id"
  key         = "CRM_WEBSITE_TOKEN"
  value       = var.website_token
}
