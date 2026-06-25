# Resource CRUD patterns

Every resource follows the same shape. Copy an existing implementation that
matches the resource's traits rather than starting blank:

- `internal/provider/resource_applications/applications_resource.go` — top-level
  (no parent), sync create, no delete endpoint (Delete is a no-op).
- `internal/provider/resource_environments/environments_resource.go` — a child
  resource (parent `application`), full CRUD.
- `internal/provider/resource_bucket_keys/bucket_keys_resource.go` — child
  resource, sensitive field (`access_key_secret`).
- `internal/provider/resource_database_clusters/database_clusters_resource.go` —
  `config` as a generic `map(string)`, a computed nested block left null.
- `internal/provider/resource_database_schemas/database_schemas_resource.go` — a
  two-level child (item path needs both the cluster and the schema id).
- `internal/provider/resource_background_processes/background_processes_resource.go`
  — a nested object input (`config`) serialized via `providerutil.AttrToAny`.
- `internal/provider/resource_deployments/deployments_resource.go` — fully
  hand-written action resource (its create has no body, so it isn't generated):
  create triggers, read fetches, Update is a passthrough, Delete is a no-op.

Shared helpers are in `internal/providerutil`; the HTTP client (`cloud.Client`,
`cloud.New`, `cloud.NotFound`, `Do(ctx, method, path, body, out)`) in
`internal/cloud`.

## The skeleton

```go
package resource_<name>

var (
	_ resource.Resource                = (*<name>Resource)(nil)
	_ resource.ResourceWithConfigure   = (*<name>Resource)(nil)
	_ resource.ResourceWithImportState = (*<name>Resource)(nil)
)

func New<Prefix>Resource() resource.Resource { return &<name>Resource{} }

type <name>Resource struct{ client *cloud.Client }

func (r *<name>Resource) Metadata(...) { resp.TypeName = req.ProviderTypeName + "_<name>" }
func (r *<name>Resource) Configure(...) { r.client = providerutil.Client(req, resp) }
func (r *<name>Resource) Schema(ctx, _, resp) {
	s := <Prefix>ResourceSchema(ctx)
	// ... schema tuning ...
	s.MarkdownDescription = "Manages a <thing> in Laravel Cloud."
	resp.Schema = s
}
// Create / Read / Update / Delete / ImportState + <x>Data{} + apply()
```

Get exact model fields/types and Required/Optional/Computed flags from the
package's `*_resource_gen.go` (`<Prefix>Model`).

## Schema tuning (`providerutil` helpers)

- **`Required(s.Attributes, parent)`** — the parent path-param attribute (e.g.
  `application`, `environment`, `instance`, `filesystem`), which the preprocessor
  injects. Makes it Required + RequiresReplace.
- **`Input(s.Attributes, names...)`** — write-only / nested inputs that come out
  Optional+Computed and aren't echoed back, so they don't end up unknown after
  apply. Handles nested (`SingleNestedAttribute`) and list types too.
- **`RequiresReplace(s.Attributes, names...)`** — create-only fields (in the
  create body but not the update body). Cloud updates are narrow (often just one
  or two fields), so most create inputs are RequiresReplace.
- **`Sensitive(s.Attributes, names...)`** — secrets/keys.
- **`UseStateForUnknown(s.Attributes, "id", "<selfid>")`** — `id` (when present)
  and the self-id field.
- **`AttrToAny(value)`** — converts a framework value (nested objects, lists,
  maps) to a plain Go value for a request body.

## Create — sync (the Cloud default)

Cloud creates are synchronous (HTTP 200/201 with the resource body), so unlike
Forge there is no async id-resolution:

```go
var data <x>Data
err := r.client.Do(ctx, http.MethodPost, collection, body, &data)
data.apply(&plan)   // sets the self-id from data.id
```

Build `body` as `map[string]any`, skipping null/unknown; nested/list inputs via
`providerutil.AttrToAny`. All Cloud ids are strings (`%s` in paths). The parent
comes from the (injected) parent attribute; the collection path embeds it, e.g.
`fmt.Sprintf("/applications/%s/environments", plan.Application.ValueString())`.

The hand-written `deployments` resource is the exception: its create has no body
(`Do(ctx, POST, endpoint, nil, &data)`), and its schema is written by hand.

## Read / Update / Delete

- **Read**: GET the item by self-id; `cloud.NotFound(err)` → `RemoveResource`.
- **Update** uses **PATCH** (`http.MethodPatch`): send the updatable fields, then
  GET and re-apply (the update response may be partial). If there's no update (or
  an empty update body), implement Update as `Get(plan)`/`Set(plan)` and mark the
  inputs RequiresReplace.
- **Delete**: DELETE the item; ignore `cloud.NotFound`. Some resources have no
  delete endpoint (`applications`, `commands`, `deployments`) — make Delete a
  no-op that returns, with a comment.

## `apply` — mapping the JSON:API wire format

```go
type <x>Data struct {
	ID         string `json:"id"`
	Attributes struct {
		Name   string  `json:"name"`
		Status *string `json:"status"` // pointer for nullable
	} `json:"attributes"`
}

func (d *<x>Data) apply(m *<Prefix>Model) {
	m.<SelfId> = types.StringValue(d.ID) // e.g. m.Environment / m.Filesystem
	m.Name = types.StringValue(d.Attributes.Name)
}
```

Every Computed-only attribute must get a known value (response value or
`types.XNull()`); never overwrite write-only inputs (keys, passwords,
polymorphic `config`) — leave the plan/state value.

## ImportState

Import id is `parent/.../selfid` (or just `selfid` with no parent). Use
`providerutil.ParseID(req.ID, n)` and `SetAttribute` each segment.

## Schema validity smoke test

Drop this in `internal/provider/`, run it, delete it:

```go
func TestZZSchemaValid(t *testing.T) {
	srv := providerserver.NewProtocol6(New("test")())()
	resp, _ := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("%s - %s", d.Summary, d.Detail)
		}
	}
}
```

## Common failure → cause

- resource skipped at generation ("schema composition not supported") → a
  discriminated `oneOf` needs `genericizeUnion` (or the property dropped).
- `"connection" is a reserved root block name` (or `provider`, etc.) → add to
  the `renames` map.
- `plan.<Parent> undefined` / missing parent attribute → add the create-request
  schema to the `parentParams` map and regenerate.
- duplicate attribute at generation → a flattened relationship collided; ensure
  relationships are dropped, not hoisted.
- stray `include` attribute → `stripParams("include")` not applied.
- `invalid result object after apply: <attr> unknown` → a write-only input needs
  `Input()`, or a Computed field isn't set in `apply`.
