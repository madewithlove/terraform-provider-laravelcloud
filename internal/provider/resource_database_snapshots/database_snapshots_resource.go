package resource_database_snapshots

// This file implements the database_snapshots resource. Unlike *_resource_gen.go
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
	_ resource.Resource                = (*databaseSnapshotsResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseSnapshotsResource)(nil)
	_ resource.ResourceWithImportState = (*databaseSnapshotsResource)(nil)
)

// NewDatabaseSnapshotsResource returns a new database_snapshots resource.
func NewDatabaseSnapshotsResource() resource.Resource {
	return &databaseSnapshotsResource{}
}

type databaseSnapshotsResource struct {
	client *cloud.Client
}

func (r *databaseSnapshotsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_snapshots"
}

func (r *databaseSnapshotsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *databaseSnapshotsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DatabaseSnapshotsResourceSchema(ctx)

	// database is the parent path parameter, only used at create time.
	providerutil.Required(s.Attributes, "database")

	// The API exposes no snapshot update endpoint, so create inputs are immutable.
	providerutil.RequiresReplace(s.Attributes, "name", "description")
	providerutil.UseStateForUnknown(s.Attributes, "id", "database_snapshot")

	s.MarkdownDescription = "Manages a snapshot of a Laravel Cloud database cluster."
	resp.Schema = s
}

func (r *databaseSnapshotsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseSnapshotsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}

	var data databaseSnapshotData
	endpoint := fmt.Sprintf("/databases/clusters/%s/snapshots", plan.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create database snapshot", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseSnapshotsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseSnapshotsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data databaseSnapshotData
	endpoint := fmt.Sprintf("/database-snapshots/%s", state.DatabaseSnapshot.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read database snapshot", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseSnapshotsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The API exposes no snapshot update endpoint and every mutable attribute
	// forces replacement, so an update simply persists the planned values.
	var plan DatabaseSnapshotsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseSnapshotsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseSnapshotsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/database-snapshots/%s", state.DatabaseSnapshot.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete database snapshot", err.Error())
	}
}

func (r *databaseSnapshotsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"database/database_snapshot\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_snapshot"), parts[1])...)
}

// databaseSnapshotData is the JSON:API representation of a database snapshot.
type databaseSnapshotData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name         string  `json:"name"`
		Description  *string `json:"description"`
		Status       *string `json:"status"`
		CompletedAt  *string `json:"completed_at"`
		CreatedAt    *string `json:"created_at"`
		PitrEnabled  *bool   `json:"pitr_enabled"`
		PitrEndsAt   *string `json:"pitr_ends_at"`
		StorageBytes *int64  `json:"storage_bytes"`
	} `json:"attributes"`
}

func (d *databaseSnapshotData) apply(m *DatabaseSnapshotsModel) {
	m.Id = types.StringValue(d.ID)
	m.DatabaseSnapshot = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)

	if d.Attributes.Description != nil {
		m.Description = types.StringValue(*d.Attributes.Description)
	} else {
		m.Description = types.StringNull()
	}
	if d.Attributes.Status != nil {
		m.Status = types.StringValue(*d.Attributes.Status)
	} else {
		m.Status = types.StringNull()
	}
	if d.Attributes.CompletedAt != nil {
		m.CompletedAt = types.StringValue(*d.Attributes.CompletedAt)
	} else {
		m.CompletedAt = types.StringNull()
	}
	if d.Attributes.CreatedAt != nil {
		m.CreatedAt = types.StringValue(*d.Attributes.CreatedAt)
	} else {
		m.CreatedAt = types.StringNull()
	}
	if d.Attributes.PitrEnabled != nil {
		m.PitrEnabled = types.BoolValue(*d.Attributes.PitrEnabled)
	} else {
		m.PitrEnabled = types.BoolNull()
	}
	if d.Attributes.PitrEndsAt != nil {
		m.PitrEndsAt = types.StringValue(*d.Attributes.PitrEndsAt)
	} else {
		m.PitrEndsAt = types.StringNull()
	}
	if d.Attributes.StorageBytes != nil {
		m.StorageBytes = types.Int64Value(*d.Attributes.StorageBytes)
	} else {
		m.StorageBytes = types.Int64Null()
	}
}
