package resource_databases

// This file implements the databases resource. Unlike *_resource_gen.go it is
// NOT regenerated, so the CRUD logic below is safe to edit.

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
	_ resource.Resource                = (*databasesResource)(nil)
	_ resource.ResourceWithConfigure   = (*databasesResource)(nil)
	_ resource.ResourceWithImportState = (*databasesResource)(nil)
)

// NewDatabasesResource returns a new databases resource.
func NewDatabasesResource() resource.Resource {
	return &databasesResource{}
}

type databasesResource struct {
	client *cloud.Client
}

func (r *databasesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_databases"
}

func (r *databasesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *databasesResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DatabasesResourceSchema(ctx)

	// Only "config" is updatable in place; the rest of the inputs are immutable.
	providerutil.RequiresReplace(s.Attributes, "name", "region", "type", "cluster_id")
	providerutil.UseStateForUnknown(s.Attributes, "id", "database")

	s.MarkdownDescription = "Manages a database in Laravel Cloud."
	resp.Schema = s
}

func (r *databasesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabasesModel
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

	var data databaseData
	if err := r.client.Do(ctx, http.MethodPost, "/databases", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create database", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databasesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabasesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data databaseData
	endpoint := fmt.Sprintf("/databases/%s", state.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read database", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databasesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatabasesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{}
	if v := providerutil.AttrToAny(plan.Config); v != nil {
		body["config"] = v
	}

	endpoint := fmt.Sprintf("/databases/%s", plan.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update database", err.Error())
		return
	}

	var data databaseData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated database", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databasesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabasesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/databases/%s", state.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete database", err.Error())
	}
}

func (r *databasesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), req.ID)...)
}

// databaseData maps the JSON:API representation. "config" is preserved from
// configuration (it is polymorphic by database type); "connection_details" is
// left null (the computed connection block is not yet mapped).
type databaseData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name      string  `json:"name"`
		Region    string  `json:"region"`
		Type      string  `json:"type"`
		Status    string  `json:"status"`
		ClusterId *int64  `json:"cluster_id"`
		CreatedAt *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *databaseData) apply(m *DatabasesModel) {
	m.Id = types.StringValue(d.ID)
	m.Database = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Region = types.StringValue(d.Attributes.Region)
	m.Type = types.StringValue(d.Attributes.Type)
	m.Status = types.StringValue(d.Attributes.Status)
	if d.Attributes.ClusterId != nil {
		m.ClusterId = types.Int64Value(*d.Attributes.ClusterId)
	}
	if d.Attributes.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*d.Attributes.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
	m.ConnectionDetails = NewConnectionDetailsValueNull()
}
