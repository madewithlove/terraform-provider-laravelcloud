# Laravel Cloud Terraform Provider

A Terraform provider for [Laravel Cloud](https://cloud.laravel.com), with
resource schemas generated from the official Laravel Cloud OpenAPI document
(`https://cloud.laravel.com/api-docs/api.json`) using the
[HashiCorp OpenAPI provider code generators](https://developer.hashicorp.com/terraform/plugin/code-generation/openapi-generator),
and hand-written CRUD on top.

## Usage

```hcl
terraform {
  required_providers {
    laravelcloud = {
      source  = "madewithlove/laravelcloud"
      version = "~> 0.1"
    }
  }
}

provider "laravelcloud" {
  # api_token may also be set via the LARAVEL_CLOUD_TOKEN environment variable.
  api_token = var.laravel_cloud_token
}

resource "laravelcloud_applications" "example" {
  name       = "my-app"
  region     = "us-east-2"
  repository = "my-org/my-app"
}
```

### Provider configuration

| Argument    | Environment             | Description                                                  |
| ----------- | ----------------------- | ------------------------------------------------------------ |
| `api_token` | `LARAVEL_CLOUD_TOKEN`   | Laravel Cloud API token (required).                          |
| `endpoint`  | `LARAVEL_CLOUD_ENDPOINT`| API base URL. Defaults to `https://cloud.laravel.com/api`.   |

## Resources

CRUD (create, read, update where supported, delete, import) is implemented for:
`applications`, `environments`, `instances`, `background_processes`,
`buckets`, `bucket_keys`, `caches`, `database_clusters`, `database_schemas`,
`database_snapshots`, `domains`, `commands`, `deployments`,
`websocket_clusters`, `websocket_applications`.

`environment_variable` manages one environment variable at a time and is
written by hand, because the API sets and deletes variables in bulk on their
environment.

The schemas are generated from the OpenAPI document; the hand-written CRUD lives
in `internal/provider/resource_*/*_resource.go`. Shared helpers are in
`internal/providerutil` and the HTTP client in `internal/cloud`. Regenerate with
`make generate`.

> The provider has not yet been validated against a live Laravel Cloud account.
> A few computed nested outputs are not yet mapped (e.g. a database's
> `connection_details`), and the polymorphic database `config` is represented as
> a `map(string)`.
