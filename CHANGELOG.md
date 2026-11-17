# Changelog

All notable changes to this provider are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/), and the provider follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- `laravelcloud_instances` is now usable for managed queues. Previously it
  failed at every stage:
  - The required `shutdown_timeout` attribute carried an
    `int64validator.Between(1, 0)` validator (min greater than max), which
    errors on construction and made `terraform validate` fail for any
    configuration using the resource. Corrected to `int64validator.AtLeast(1)`
    in `provider_code_spec.json` so it survives regeneration.
  - The create/update request always sent `min_replicas`/`max_replicas` and
    never sent `polling_interval`/`visibility_timeout`/`shutdown_timeout`. The
    API rejects `min_replicas` for managed queues and requires the queue-timing
    fields. The payload now branches on `type`.
  - The computed attributes `is_default`, `paused`, `queue_status`,
    `sleep_with_app` (and the queue-timing fields) were not mapped back from the
    API response, producing "invalid result object after apply" errors. They are
    now mapped.
  - The nested `background_processes` attributes were `Optional + Computed`;
    because the API does not echo background processes, unset fields stayed
    unknown after apply and triggered the same error. They are now `Optional`.

## [0.1.0]

Initial release. Generated from the Laravel Cloud OpenAPI document with
hand-written CRUD.

### Added

- Provider configuration via `api_token` / `endpoint` (or `LARAVEL_CLOUD_TOKEN`
  / `LARAVEL_CLOUD_ENDPOINT`).
- Resources: `applications`, `environments`, `instances`,
  `background_processes`, `buckets`, `bucket_keys`, `caches`,
  `database_clusters`, `database_schemas`, `database_snapshots`, `domains`,
  `commands`, `deployments`, `websocket_clusters`, `websocket_applications`,
  each with `terraform import` support.
- A reproducible code-generation pipeline (`make generate`) driven by the full
  OpenAPI document at `https://cloud.laravel.com/api-docs/api.json`.
