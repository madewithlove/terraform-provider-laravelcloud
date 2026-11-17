package resource_database_clusters

// This file implements the database_clusters resource. Unlike *_resource_gen.go
// it is NOT regenerated, so the CRUD logic below is safe to edit.

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"

	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

var (
	_ resource.Resource                = (*databaseClustersResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseClustersResource)(nil)
	_ resource.ResourceWithImportState = (*databaseClustersResource)(nil)
)

// NewDatabaseClustersResource returns a new database_clusters resource.
func NewDatabaseClustersResource() resource.Resource {
	return &databaseClustersResource{}
}

type databaseClustersResource struct {
	client *cloud.Client
}

func (r *databaseClustersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_clusters"
}

func (r *databaseClustersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *databaseClustersResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DatabaseClustersResourceSchema(ctx)

	// The update endpoint only accepts "config"; every other create input is
	// immutable in place and forces replacement.
	providerutil.RequiresReplace(s.Attributes, "cluster_id", "name", "region", "type")
	// cluster_id is optional and computed and forces replacement: without
	// UseStateForUnknown, leaving it unset would plan it as unknown on every
	// change and replace the cluster.
	providerutil.UseStateForUnknown(s.Attributes, "id", "database", "cluster_id", "created_at")

	s.MarkdownDescription = "Manages a database cluster in Laravel Cloud."
	resp.Schema = s
}

func (r *databaseClustersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseClustersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"type":   plan.Type.ValueString(),
		"name":   plan.Name.ValueString(),
		"region": plan.Region.ValueString(),
	}
	if v := configBody(plan.Config); v != nil {
		body["config"] = v
	}
	if !plan.ClusterId.IsNull() && !plan.ClusterId.IsUnknown() {
		body["cluster_id"] = plan.ClusterId.ValueInt64()
	}

	var data databaseClusterData
	if err := r.client.Do(ctx, http.MethodPost, "/databases/clusters", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create database cluster", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseClustersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseClustersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data databaseClusterData
	endpoint := fmt.Sprintf("/databases/clusters/%s", state.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read database cluster", err.Error())
		return
	}

	data.apply(&state)
	// Only a read maps config back: after create or update the state must
	// match the plan, and the API may normalize values (such as sizes).
	if data.Attributes.Config != nil {
		state.Config = clusterConfig(data.Attributes.Config, state.Config)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseClustersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatabaseClustersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The update endpoint only accepts "config".
	body := map[string]any{}
	if v := configBody(plan.Config); v != nil {
		body["config"] = v
	}

	var data databaseClusterData
	endpoint := fmt.Sprintf("/databases/clusters/%s", plan.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to update database cluster", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseClustersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseClustersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/databases/clusters/%s", state.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete database cluster", err.Error())
	}
}

func (r *databaseClustersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"database\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), parts[0])...)
}

// databaseClusterData is the JSON:API representation of a database cluster.
type databaseClusterData struct {
	ID         string `json:"id"`
	Attributes struct {
		ClusterId *int64         `json:"cluster_id"`
		Name      string         `json:"name"`
		Region    string         `json:"region"`
		Type      string         `json:"type"`
		Status    *string        `json:"status"`
		CreatedAt *string        `json:"created_at"`
		Config    map[string]any `json:"config"`
	} `json:"attributes"`
}

func (d *databaseClusterData) apply(m *DatabaseClustersModel) {
	a := d.Attributes
	m.Id = types.StringValue(d.ID)
	m.Database = types.StringValue(d.ID)
	m.Name = types.StringValue(a.Name)
	m.Region = types.StringValue(a.Region)
	m.Type = types.StringValue(clusterType(a.Type, m.Type))
	m.Status = providerutil.OptionalString(a.Status)
	m.CreatedAt = providerutil.OptionalString(a.CreatedAt)

	// The API does not echo cluster_id; keep what was configured.
	if a.ClusterId != nil {
		m.ClusterId = types.Int64Value(*a.ClusterId)
	} else {
		m.ClusterId = providerutil.KnownInt64(m.ClusterId)
	}

	// connection_details is a computed nested block we do not map back; leave it
	// null so the value is known.
	m.ConnectionDetails = NewConnectionDetailsValueNull()
}

// versionSuffix matches the engine version the create request appends to a
// cluster type, as in "laravel_mysql_84" or "neon_serverless_postgres_17".
var versionSuffix = regexp.MustCompile(`_[0-9]+$`)

// clusterType reconciles the type the API reports with the configured one.
// The API reports the engine family without its version ("laravel_mysql"),
// while a cluster may have been created with a versioned type
// ("laravel_mysql_84"). When the configured type belongs to the reported
// family, keep the configured value so it neither drifts nor forces a
// replacement.
func clusterType(reported string, prior types.String) string {
	if prior.IsNull() || prior.IsUnknown() {
		return reported
	}
	configured := prior.ValueString()
	if configured == reported || versionSuffix.ReplaceAllString(configured, "") == reported {
		return configured
	}
	return reported
}

// clusterConfig converts the API's typed config object into the string map the
// schema exposes. When there is a prior config (from the plan or state), only
// its keys are kept, so a configuration listing a subset of the settings does
// not drift; on import every non-null setting is read.
func clusterConfig(reported map[string]any, prior types.Map) types.Map {
	values := map[string]attr.Value{}
	if !prior.IsNull() && !prior.IsUnknown() {
		for key, value := range prior.Elements() {
			if v, ok := reported[key]; ok && v != nil {
				values[key] = types.StringValue(configString(v))
			} else {
				values[key] = value
			}
		}
	} else {
		for key, v := range reported {
			if v != nil {
				values[key] = types.StringValue(configString(v))
			}
		}
	}
	return types.MapValueMust(types.StringType, values)
}

func configString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		encoded, _ := json.Marshal(t)
		return string(encoded)
	}
}

// configBody converts the configured string map into a request body, sending
// booleans and numbers as JSON booleans and numbers. The API validates config
// settings by type and rejects, for example, the string "false" for a boolean.
func configBody(config types.Map) map[string]any {
	if config.IsNull() || config.IsUnknown() {
		return nil
	}
	body := map[string]any{}
	for key, value := range config.Elements() {
		s, ok := value.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		body[key] = typedConfigValue(s.ValueString())
	}
	return body
}

func typedConfigValue(s string) any {
	if s == "true" || s == "false" {
		return s == "true"
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
