package resource_filesystem_keys

// This file implements the filesystem_keys resource. Unlike *_resource_gen.go it
// is NOT regenerated, so the CRUD logic below is safe to edit.

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
	_ resource.Resource                = (*filesystemKeysResource)(nil)
	_ resource.ResourceWithConfigure   = (*filesystemKeysResource)(nil)
	_ resource.ResourceWithImportState = (*filesystemKeysResource)(nil)
)

// NewFilesystemKeysResource returns a new filesystem_keys resource.
func NewFilesystemKeysResource() resource.Resource {
	return &filesystemKeysResource{}
}

type filesystemKeysResource struct {
	client *cloud.Client
}

func (r *filesystemKeysResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_filesystem_keys"
}

func (r *filesystemKeysResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *filesystemKeysResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := FilesystemKeysResourceSchema(ctx)

	// filesystem is the parent path parameter.
	providerutil.Required(s.Attributes, "filesystem")
	// permission can only be set at creation; only name is updatable in place.
	providerutil.RequiresReplace(s.Attributes, "permission")
	providerutil.Sensitive(s.Attributes, "access_key_secret")
	providerutil.UseStateForUnknown(s.Attributes, "id", "filesystem_key")

	s.MarkdownDescription = "Manages a filesystem key in Laravel Cloud."
	resp.Schema = s
}

func (r *filesystemKeysResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FilesystemKeysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"permission": plan.Permission.ValueString(),
	}

	var data filesystemKeyData
	collection := fmt.Sprintf("/filesystems/%s/keys", plan.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, collection, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create filesystem key", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *filesystemKeysResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FilesystemKeysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data filesystemKeyData
	endpoint := fmt.Sprintf("/filesystem-keys/%s", state.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read filesystem key", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *filesystemKeysResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FilesystemKeysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}

	endpoint := fmt.Sprintf("/filesystem-keys/%s", plan.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update filesystem key", err.Error())
		return
	}

	var data filesystemKeyData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated filesystem key", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *filesystemKeysResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FilesystemKeysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/filesystem-keys/%s", state.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete filesystem key", err.Error())
	}
}

func (r *filesystemKeysResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"filesystem/filesystem_key\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem_key"), parts[1])...)
}

// filesystemKeyData is the JSON:API representation of a filesystem key.
type filesystemKeyData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name            string  `json:"name"`
		Permission      string  `json:"permission"`
		AccessKeyId     *string `json:"access_key_id"`
		AccessKeySecret *string `json:"access_key_secret"`
		CreatedAt       *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *filesystemKeyData) apply(m *FilesystemKeysModel) {
	m.Id = types.StringValue(d.ID)
	m.FilesystemKey = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Permission = types.StringValue(d.Attributes.Permission)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)

	// The access key id and secret are only returned when the key is created;
	// subsequent reads omit the secret, so retain the previously stored values
	// when the API does not echo them back.
	if d.Attributes.AccessKeyId != nil {
		m.AccessKeyId = types.StringValue(*d.Attributes.AccessKeyId)
	} else if m.AccessKeyId.IsUnknown() {
		m.AccessKeyId = types.StringNull()
	}
	if d.Attributes.AccessKeySecret != nil {
		m.AccessKeySecret = types.StringValue(*d.Attributes.AccessKeySecret)
	} else if m.AccessKeySecret.IsUnknown() {
		m.AccessKeySecret = types.StringNull()
	}
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
