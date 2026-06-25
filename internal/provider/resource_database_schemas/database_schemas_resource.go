package resource_database_schemas

// This file implements the database_schemas resource. Unlike *_resource_gen.go
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
	_ resource.Resource                = (*databaseSchemasResource)(nil)
	_ resource.ResourceWithConfigure   = (*databaseSchemasResource)(nil)
	_ resource.ResourceWithImportState = (*databaseSchemasResource)(nil)
)

// NewDatabaseSchemasResource returns a new database_schemas resource.
func NewDatabaseSchemasResource() resource.Resource {
	return &databaseSchemasResource{}
}

type databaseSchemasResource struct {
	client *cloud.Client
}

func (r *databaseSchemasResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_database_schemas"
}

func (r *databaseSchemasResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *databaseSchemasResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DatabaseSchemasResourceSchema(ctx)

	// database is the parent path parameter (a database cluster identifier).
	providerutil.Required(s.Attributes, "database")

	// The API exposes no schema update endpoint, so "name" is immutable in place.
	providerutil.RequiresReplace(s.Attributes, "name")
	providerutil.UseStateForUnknown(s.Attributes, "id", "schema")

	s.MarkdownDescription = "Manages a database (schema) within a Laravel Cloud database cluster."
	resp.Schema = s
}

func (r *databaseSchemasResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatabaseSchemasModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}

	var data databaseSchemaData
	endpoint := fmt.Sprintf("/databases/clusters/%s/databases", plan.Database.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create database schema", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseSchemasResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatabaseSchemasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data databaseSchemaData
	endpoint := fmt.Sprintf("/databases/clusters/%s/databases/%s", state.Database.ValueString(), state.Schema.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read database schema", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseSchemasResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The API exposes no schema update endpoint and every mutable attribute forces
	// replacement, so an update simply persists the planned values.
	var plan DatabaseSchemasModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseSchemasResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatabaseSchemasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/databases/clusters/%s/databases/%s", state.Database.ValueString(), state.Schema.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete database schema", err.Error())
	}
}

func (r *databaseSchemasResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"database/schema\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("schema"), parts[1])...)
}

// databaseSchemaData is the JSON:API representation of a database schema.
type databaseSchemaData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name      string  `json:"name"`
		Status    *string `json:"status"`
		CreatedAt *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *databaseSchemaData) apply(m *DatabaseSchemasModel) {
	m.Id = types.StringValue(d.ID)
	m.Schema = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)

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
}
