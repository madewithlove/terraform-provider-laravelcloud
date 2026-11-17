# Live validation against the Laravel Cloud API

Compile/vet/lint and the schema smoke test catch structural problems, but only a
real apply confirms the request bodies and response mapping. Validate at least
one representative resource end-to-end after non-trivial changes.

## Prerequisites

- A Laravel Cloud **API token** (Bearer). Keep it out of git — put it in a
  gitignored file (e.g. `.cloud_token`) and read it into `LARAVEL_CLOUD_TOKEN`.
- The token's organization is implicit (Cloud is not org-scoped in the path).

## Discover what the token can reach

```sh
TOKEN=$(tr -d '\n' < .cloud_token)
curl -s -H "Authorization: Bearer $TOKEN" -H "Accept: application/json" \
  https://cloud.laravel.com/api/applications      # your applications
```

Most resources hang off an application/environment, so list those first to get
ids for parent attributes.

## Harness (dev override, no install)

```sh
go build -o "$(go env GOPATH)/bin/terraform-provider-laravelcloud" .

cat > /tmp/dev.tfrc <<EOF
provider_installation {
  dev_overrides { "madewithlove/laravelcloud" = "$(go env GOPATH)/bin" }
  direct {}
}
EOF
export TF_CLI_CONFIG_FILE=/tmp/dev.tfrc
export LARAVEL_CLOUD_TOKEN=$(tr -d '\n' < .cloud_token)
```

Write a small config in a gitignored scratch dir and run the lifecycle:

```sh
terraform apply -auto-approve      # create
terraform plan                     # expect "No changes"
# edit a mutable attr (PATCH-able), then:
terraform apply -auto-approve      # update should be in-place
terraform destroy -auto-approve    # delete
```

## What to assert

- **Create** succeeds and state is populated (the self-id is set).
- **Read**: a follow-up `plan` shows no changes (mapping round-trips). A computed
  status settling is benign, not drift.
- **Update** (PATCH) of a mutable attribute is `update in-place`, not replace.
- **Delete** removes it on the API (verify with `curl`). Note `applications`,
  `commands`, and `deployments` have no delete endpoint — their Delete is a
  state-only no-op by design.

## Good resources to validate

Prefer cheap, low-side-effect resources. `applications` and `environments` are
the natural entry points. Avoid provisioning real compute/storage
(`instances`, `databases`, `filesystems`) unless you intend to incur cost, and
clean them up promptly.

## Known gaps to keep in mind while validating

- `databases.connection_details` is currently left null (the computed nested
  block isn't mapped yet).
- `databases.config` is read back on refresh only for the keys configured
  (all keys on import); `background_processes.config` is read back in full.
- Prefer import-only checks against existing resources: `TestAccImportExisting`
  plans an import with generated configuration and asserts no changes, and
  never creates or destroys anything (see the comment on the test for the
  environment variables).

## Cleanup

Delete what you created and remove the scratch dir and dev-override file. If an
apply fails mid-create, the resource may still exist on the API — delete it with
`curl -X DELETE`.
