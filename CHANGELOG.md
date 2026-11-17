# Changelog

All notable changes to this provider are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/), and the provider follows
[Semantic Versioning](https://semver.org/).

## [0.2.0] — Unreleased

Existing Laravel Cloud resources can now be imported and adopted: applications,
environments, app instances, background processes, Laravel MySQL and Neon
clusters, and domains import with a plan that shows no changes.

### Added

- `instances`: `type` accepts `app` and `queue`, the types of instances
  Laravel Cloud creates itself, so an environment's app instance can be
  imported and managed.
- `database_clusters`: `type` accepts the unversioned engine names the API
  reports (`laravel_mysql`, `aws_rds_mysql`, `aws_rds_postgres`,
  `neon_serverless_postgres`). A versioned type (`laravel_mysql_84`) stays as
  configured when the API reports its engine family.
- `domains`: `cloudflare_strategy`, `allow_downtime` and `verification_method`
  are sent on create, and `dns_records` exposes the records Laravel Cloud asks
  for (`dcv`, `origin`, `origin_cname`, `pre_verification` as
  `{type, name, value}` objects, and `ssl`).
- `applications`: `source_control_provider_type` and `cluster_id` are sent on
  create.
- Import tests that run Terraform against a fake API, a live acceptance test
  (`TestAccImportExisting`) that imports existing resources without changing
  them, and a CI job running the tests.

### Fixed

- `applications`: reading failed because the API returns `repository` as an
  object; it now maps to the repository's full name.
- `domains`: every read failed with a value conversion error; the resource now
  uses the full schema.
- `environments`: `branch` was read as empty, which planned a replacement after
  an import. It is now read from the sideloaded branch (`?include=branch`).
- `instances` is usable for managed queues and imports app instances:
  - `shutdown_timeout` carried `int64validator.Between(1, 0)`, which fails on
    construction, so `terraform validate` failed for any configuration using
    the resource. It is now `AtLeast(1)`.
  - `polling_interval`, `visibility_timeout`, `shutdown_timeout` and
    `min_replicas` are optional; the queue settings only apply to managed
    queues and are null for app and service instances.
  - The request payload depends on `type`: managed queues send the queue
    settings and never `min_replicas`, which the API rejects for them; other
    types send `min_replicas` and no queue settings. Unset optional settings
    are no longer sent as zero values.
  - Every attribute the API returns (`is_default`, `paused`, `queue_status`,
    `sleep_with_app`, the queue settings, ...) is read, so none stay unknown
    after apply.
  - The nested `background_processes` attributes are optional but not
    computed: the API does not echo them, so unset fields stayed unknown
    after apply.
- `database_clusters` and `background_processes`: `config` is read back, so an
  import matches the configuration and drift is detected. Cluster settings are
  sent as JSON booleans and numbers instead of strings.
- `database_clusters` and `domains`: changing any attribute while an optional
  replacement-forcing attribute (`cluster_id`, `cloudflare_strategy`,
  `wildcard_enabled`, `www_redirect`) was unset planned to replace the
  resource. Those attributes now keep their state value.

### Notes

- The schema is still generated from the OpenAPI document pinned in
  `generator/docs.openapi.json`. Where it disagrees with the live API, the
  preprocessor now corrects it (`generator/preprocess/schema_fixes.go`), so
  `make generate` reproduces the fixes. Schema changes the generators cannot
  express are applied in the hand-written resource (`background_processes`
  on instances).
- A cluster `config` listing only some settings shows one in-place update
  after an import (the import reads every setting); it is stable afterwards.

## [0.1.0] — 2026-06-25

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
