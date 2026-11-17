resource "laravelcloud_domains" "example" {
  environment         = "environment-id"
  name                = "app.example.com"
  cloudflare_strategy = "dns_proxy"
}

# The records Laravel Cloud asks for, e.g. to create them at your DNS provider.
output "origin_cname" {
  value = laravelcloud_domains.example.dns_records.origin_cname
}
