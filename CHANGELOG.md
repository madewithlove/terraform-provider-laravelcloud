# Changelog

All notable changes to this provider are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/), and the provider follows
[Semantic Versioning](https://semver.org/).

## [Unreleased] — 0.1.0

Initial release. Generated from the Laravel Cloud OpenAPI document with
hand-written CRUD.

### Added

- Provider configuration via `api_token` / `endpoint` (or `LARAVEL_CLOUD_TOKEN`
  / `LARAVEL_CLOUD_ENDPOINT`).
- Resources: `applications`, `environments`, `instances`, `databases`,
  `domains`, `filesystems`, `filesystem_keys`, `background_processes`,
  `commands`, `deployments`, each with `terraform import` support.
- A reproducible code-generation pipeline (`make generate`).
