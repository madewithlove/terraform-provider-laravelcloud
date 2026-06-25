---
name: update-laravelcloud-provider
description: >-
  Regenerate and extend the Laravel Cloud Terraform provider from its OpenAPI
  document. Use this whenever the Laravel Cloud API changes, the
  `generator/docs.openapi.json` spec is refreshed, a new Cloud resource needs a
  Terraform resource, or generated schemas/CRUD need to be rebuilt — even when
  phrased as "the Cloud API added X", "regenerate the provider", "the spec
  changed", or "add a <thing> resource". It encodes the spec-preprocessing
  rules, the CRUD implementation pattern, live validation against Laravel Cloud,
  and the CI specifics this repo depends on, so the work is repeatable instead
  of rediscovered.
---

# Updating the Laravel Cloud Terraform provider

This provider's resource **schemas** are generated from the Laravel Cloud
OpenAPI document; the **CRUD** logic is hand-written on top. The generators only
emit schemas + models — never CRUD — so adding or changing a resource is always
two moves: regenerate the schema, then write/adjust the CRUD.

The Cloud API is **JSON:API**, which is hostile to the code generators in
specific, recurring ways that the preprocessor (`generator/preprocess`)
neutralizes before generation. Understanding why each rule exists is the key to
this skill — see "Preprocessing rules" and `references/crud-patterns.md`.

The spec lives at `https://cloud.laravel.com/api-docs/api.json` (OpenAPI 3.1). The
API base is `https://cloud.laravel.com/api`, auth is a Bearer token, and — unlike
Forge — resources are **not** organization-scoped in the path (the token implies
the organization).

## When the API updates — the loop

1. **Refresh the spec**: `make fetch-spec` (downloads `https://cloud.laravel.com/api-docs/api.json` to `generator/docs.openapi.json`).
2. **Diff it**: `git diff generator/docs.openapi.json` — new paths, schemas, renamed/removed fields.
3. **Update `generator/generator_config.yml`** for new resources or path/method changes (one block per resource: create/read/update/delete). Note Cloud uses **PATCH** for updates.
4. **Update the preprocessor** if the change introduces a new recurring hazard (see "Preprocessing rules").
5. **Regenerate**: `make generate` (preprocess → tfplugingen-openapi → tfplugingen-framework → scaffold → gofmt → docs). Install tools once with `make tools`.
6. **Wire new resources** into `internal/provider/provider.go` `Resources()`. The scaffolder stubs a `*_resource.go` but does not touch `provider.go`.
7. **Implement/adjust CRUD** per `references/crud-patterns.md`.
8. **Validate** (see "Verifying" and `references/live-validation.md`).
9. **Docs + examples** for new resources.
10. **Commit** in logical units.

The scaffolder is idempotent and never overwrites an existing `*_resource.go`.

## Preprocessing rules

`generator/preprocess/main.go` rewrites the spec before generation. Each rule
exists because of a concrete failure; extend the matching table rather than
hand-editing generated code:

- **Data-envelope unwrap** — JSON:API `{ "data": ... }` responses/relationships
  are unwrapped (by shape; no maintenance).
- **Resource-identifier collapse** — `{id, type}` objects become plain string
  ids (by shape).
- **Resource flattening** — `attributes` are hoisted to the top level and
  `type`/`links`/**`relationships`** are dropped. Relationships are dropped (not
  hoisted) because Cloud relationship members frequently share a name with a
  request/response field (e.g. an instance's `background_processes`), which would
  collide into duplicate attributes. Parent references come from path params
  instead (see below).
- **`stripParams("include")`** — the JSON:API `include` sideload query parameter
  is removed so it doesn't become a junk attribute on every resource.
- **`genericizeUnion`** — a discriminated polymorphic schema (a `oneOf`/`anyOf`
  with a `discriminator`, e.g. a database's type-specific `config`) is collapsed
  to a `map(string)`; the generators can't map composition and would otherwise
  skip the whole resource. Nullability unions (no discriminator) are left alone.
- **`renames` map** — fields colliding with reserved Terraform names. Currently
  `connection` → `connection_details` on `DatabaseResource` (`connection` is a
  reserved root block name). Add entries (e.g. `provider`, `count`, `for_each`)
  as needed.
- **`parentParams` map** — Cloud read paths are flat (`GET /environments/{environment}`),
  so the **parent** of a nested resource (e.g. `application` for an environment,
  which only appears in the create path `POST /applications/{application}/environments`)
  is never derived as an attribute. This injects the parent as a string property
  on the **create-request** schema (the generators build the model primarily from
  the create request), so it appears in the model. The resource then marks it
  Required + RequiresReplace. Add an entry when a new nested resource is added.
- **`responseFixes` map** — corrects wrong `$ref`s in the upstream spec (none
  currently needed).

After regenerating, a stray `data`/`include` attribute, a missing parent, a
skipped resource, or a reserved-word error are all signs a table needs an entry.

## Generator quirks to expect

- The generator builds the resource model primarily from the **create request
  body**. Response-only computed fields are not always merged in, so some
  resources lack `id`/`created_at`/`status`; the self-id field (named after the
  read path param, e.g. `environment`, `filesystem`) serves as the identifier.
- The live API still returns the nested JSON:API envelope (`data.attributes.*`)
  even though the TF schema is flat — CRUD bridges the two by hand.

## Verifying

```sh
go build ./...
go vet ./...
golangci-lint run ./...          # v2; config is .golangci.yml
go generate ./... && git diff --exit-code   # docs must be committed
```

Confirm every resource produces a valid schema (catches reserved words,
duplicate attributes, unknown-after-apply) with the `GetProviderSchema`
smoke-test in `references/crud-patterns.md`. For real confidence, validate
against the live API (`references/live-validation.md`).

## CI specifics (don't regress)

- **golangci-lint v2** config (`version: "2"`, `linters.default: none`, `gofmt`
  under `formatters`, `linters.exclusions.generated`). Generated `*_resource_gen.go`
  are skipped by their `DO NOT EDIT` header; hand-written `*_resource.go` is linted.
- **setup-terraform** uses `terraform_wrapper: false` so `tfplugindocs` can read
  `terraform ... -json` output during `go generate`.
- Actions are pinned to Node-24 releases.

## Versioning

Published to the Terraform Registry from `v*` git tags via GoReleaser; semver.
A spec change that renames/removes attributes or resources is breaking — bump
the major version and note it in `CHANGELOG.md`.
