package resource_database_clusters

// This file implements the database_clusters resource. Unlike *_resource_gen.go
// it is NOT regenerated, so the CRUD logic below is safe to edit.

import (
	"context"
	"fmt"
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
	providerutil.UseStateForUnknown(s.Attributes, "id", "database")

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
	if v := providerutil.AttrToAny(plan.Config); v != nil {
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
	if v := providerutil.AttrToAny(plan.Config); v != nil {
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

// databaseClusterData is the JSON:API representation of a database cluster. The
// "config" block is a polymorphic write-only input preserved from configuration,
// not mapped back here.
type databaseClusterData struct {
	ID         string `json:"id"`
	Attributes struct {
		ClusterId *int64  `json:"cluster_id"`
		Name      string  `json:"name"`
		Region    string  `json:"region"`
		Type      string  `json:"type"`
		Status    *string `json:"status"`
		CreatedAt *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *databaseClusterData) apply(m *DatabaseClustersModel) {
	m.Id = types.StringValue(d.ID)
	m.Database = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Region = types.StringValue(d.Attributes.Region)
	m.Type = types.StringValue(d.Attributes.Type)

	if d.Attributes.ClusterId != nil {
		m.ClusterId = types.Int64Value(*d.Attributes.ClusterId)
	} else {
		m.ClusterId = types.Int64Null()
	}
	if d.Attributes.Status != nil {
		m.Status = types.StringValue(*d.Attributes.Status)
	} else {
		m.Status = types.StringNull()
	}
	if d.Attributes.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*d.Attributes.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}

	// connection_details is a computed nested block we do not map back; leave it
	// null so the value is known. "config" is preserved from the plan/state.
	m.ConnectionDetails = NewConnectionDetailsValueNull()
}
