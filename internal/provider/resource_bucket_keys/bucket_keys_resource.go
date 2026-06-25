package resource_bucket_keys

// This file implements the bucket_keys resource. Unlike *_resource_gen.go it is
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
	_ resource.Resource                = (*bucketKeysResource)(nil)
	_ resource.ResourceWithConfigure   = (*bucketKeysResource)(nil)
	_ resource.ResourceWithImportState = (*bucketKeysResource)(nil)
)

// NewBucketKeysResource returns a new bucket_keys resource.
func NewBucketKeysResource() resource.Resource {
	return &bucketKeysResource{}
}

type bucketKeysResource struct {
	client *cloud.Client
}

func (r *bucketKeysResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket_keys"
}

func (r *bucketKeysResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *bucketKeysResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := BucketKeysResourceSchema(ctx)

	// filesystem is the parent path parameter; permission is create-only.
	providerutil.Required(s.Attributes, "filesystem")
	providerutil.RequiresReplace(s.Attributes, "permission")
	providerutil.Sensitive(s.Attributes, "access_key_secret")
	// access_key_id and access_key_secret are returned only at create time, so
	// keep their prior values during planning instead of "(known after apply)".
	providerutil.UseStateForUnknown(s.Attributes, "id", "filesystem_key", "access_key_id", "access_key_secret")

	s.MarkdownDescription = "Manages an access key for an object storage bucket in Laravel Cloud."
	resp.Schema = s
}

func (r *bucketKeysResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketKeysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"permission": plan.Permission.ValueString(),
	}

	var data bucketKeyData
	collection := fmt.Sprintf("/buckets/%s/keys", plan.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, collection, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create bucket key", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketKeysResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketKeysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data bucketKeyData
	endpoint := fmt.Sprintf("/bucket-keys/%s", state.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read bucket key", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketKeysResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketKeysModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}

	endpoint := fmt.Sprintf("/bucket-keys/%s", plan.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update bucket key", err.Error())
		return
	}

	var data bucketKeyData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated bucket key", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketKeysResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketKeysModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/bucket-keys/%s", state.FilesystemKey.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete bucket key", err.Error())
	}
}

func (r *bucketKeysResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"filesystem/filesystem_key\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem_key"), parts[1])...)
}

// bucketKeyData is the JSON:API representation of a bucket key. The secret and
// access key id are returned only at create time, so they are preserved from
// state when absent on a subsequent read.
type bucketKeyData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name            string  `json:"name"`
		Permission      string  `json:"permission"`
		AccessKeyId     *string `json:"access_key_id"`
		AccessKeySecret *string `json:"access_key_secret"`
		CreatedAt       *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *bucketKeyData) apply(m *BucketKeysModel) {
	m.Id = types.StringValue(d.ID)
	m.FilesystemKey = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Permission = types.StringValue(d.Attributes.Permission)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
	// The access key id and secret are only returned at create time; keep the
	// existing state value when the API omits them.
	if d.Attributes.AccessKeyId != nil {
		m.AccessKeyId = types.StringValue(*d.Attributes.AccessKeyId)
	}
	if d.Attributes.AccessKeySecret != nil {
		m.AccessKeySecret = types.StringValue(*d.Attributes.AccessKeySecret)
	}
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
