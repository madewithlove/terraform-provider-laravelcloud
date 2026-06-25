package resource_filesystems

// This file implements the filesystems resource. Unlike *_resource_gen.go it is
// NOT regenerated, so the CRUD logic below is safe to edit.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

var (
	_ resource.Resource                = (*filesystemsResource)(nil)
	_ resource.ResourceWithConfigure   = (*filesystemsResource)(nil)
	_ resource.ResourceWithImportState = (*filesystemsResource)(nil)
)

// NewFilesystemsResource returns a new filesystems resource.
func NewFilesystemsResource() resource.Resource {
	return &filesystemsResource{}
}

type filesystemsResource struct {
	client *cloud.Client
}

func (r *filesystemsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystems"
}

func (r *filesystemsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *filesystemsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := FilesystemsResourceSchema(ctx)

	// key_name, key_permission and jurisdiction are write-only inputs honored
	// only at creation time; they are not part of the update body.
	providerutil.Input(s.Attributes, "key_name", "key_permission")
	providerutil.RequiresReplace(s.Attributes, "key_name", "key_permission", "jurisdiction")
	providerutil.UseStateForUnknown(s.Attributes, "filesystem")

	s.MarkdownDescription = "Manages a filesystem in Laravel Cloud."
	resp.Schema = s
}

func (r *filesystemsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilesystemsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":           plan.Name.ValueString(),
		"visibility":     plan.Visibility.ValueString(),
		"jurisdiction":   plan.Jurisdiction.ValueString(),
		"key_name":       plan.KeyName.ValueString(),
		"key_permission": plan.KeyPermission.ValueString(),
	}
	if origins := providerutil.AttrToAny(plan.AllowedOrigins); origins != nil {
		body["allowed_origins"] = origins
	}

	var data filesystemData
	if err := r.client.Do(ctx, http.MethodPost, "/filesystems", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create filesystem", err.Error())
		return
	}

	resp.Diagnostics.Append(data.apply(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *filesystemsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilesystemsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data filesystemData
	endpoint := fmt.Sprintf("/filesystems/%s", state.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read filesystem", err.Error())
		return
	}

	resp.Diagnostics.Append(data.apply(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *filesystemsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FilesystemsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"visibility": plan.Visibility.ValueString(),
	}
	if origins := providerutil.AttrToAny(plan.AllowedOrigins); origins != nil {
		body["allowed_origins"] = origins
	}

	endpoint := fmt.Sprintf("/filesystems/%s", plan.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update filesystem", err.Error())
		return
	}

	var data filesystemData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated filesystem", err.Error())
		return
	}

	resp.Diagnostics.Append(data.apply(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *filesystemsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilesystemsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/filesystems/%s", state.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete filesystem", err.Error())
	}
}

func (r *filesystemsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"filesystem\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem"), parts[0])...)
}

// filesystemData is the JSON:API representation of a filesystem. key_name and
// key_permission are write-only and not returned.
type filesystemData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name           string    `json:"name"`
		Visibility     string    `json:"visibility"`
		Jurisdiction   string    `json:"jurisdiction"`
		AllowedOrigins *[]string `json:"allowed_origins"`
	} `json:"attributes"`
}

func (d *filesystemData) apply(ctx context.Context, m *FilesystemsModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.Filesystem = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Visibility = types.StringValue(d.Attributes.Visibility)
	m.Jurisdiction = types.StringValue(d.Attributes.Jurisdiction)

	if d.Attributes.AllowedOrigins == nil {
		m.AllowedOrigins = types.ListNull(types.StringType)
	} else {
		list, d2 := types.ListValueFrom(ctx, types.StringType, *d.Attributes.AllowedOrigins)
		diags.Append(d2...)
		m.AllowedOrigins = list
	}
	return diags
}
