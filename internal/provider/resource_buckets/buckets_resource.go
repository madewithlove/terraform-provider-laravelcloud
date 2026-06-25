package resource_buckets

// This file implements the buckets resource. Unlike *_resource_gen.go it is NOT
// regenerated, so the CRUD logic below is safe to edit.

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
	_ resource.Resource                = (*bucketsResource)(nil)
	_ resource.ResourceWithConfigure   = (*bucketsResource)(nil)
	_ resource.ResourceWithImportState = (*bucketsResource)(nil)
)

// NewBucketsResource returns a new buckets resource.
func NewBucketsResource() resource.Resource {
	return &bucketsResource{}
}

type bucketsResource struct {
	client *cloud.Client
}

func (r *bucketsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_buckets"
}

func (r *bucketsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *bucketsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := BucketsResourceSchema(ctx)

	// key_name, key_permission and jurisdiction are write-only, create-only
	// inputs (never echoed back), so make them plain inputs that force
	// replacement when changed. cors_settings and allowed_origins are write-only
	// inputs preserved from configuration.
	providerutil.Input(s.Attributes, "key_name", "key_permission", "jurisdiction", "cors_settings", "allowed_origins")
	providerutil.RequiresReplace(s.Attributes, "key_name", "key_permission", "jurisdiction")
	providerutil.UseStateForUnknown(s.Attributes, "id", "filesystem")

	s.MarkdownDescription = "Manages an object storage bucket in Laravel Cloud."
	resp.Schema = s
}

func (r *bucketsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BucketsModel
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
	if v := providerutil.AttrToAny(plan.AllowedOrigins); v != nil {
		body["allowed_origins"] = v
	}
	if v := providerutil.AttrToAny(plan.CorsSettings); v != nil {
		body["cors_settings"] = v
	}

	var data bucketData
	if err := r.client.Do(ctx, http.MethodPost, "/buckets", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create bucket", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BucketsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data bucketData
	endpoint := fmt.Sprintf("/buckets/%s", state.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read bucket", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BucketsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"visibility": plan.Visibility.ValueString(),
	}
	if v := providerutil.AttrToAny(plan.AllowedOrigins); v != nil {
		body["allowed_origins"] = v
	}
	if v := providerutil.AttrToAny(plan.CorsSettings); v != nil {
		body["cors_settings"] = v
	}

	endpoint := fmt.Sprintf("/buckets/%s", plan.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update bucket", err.Error())
		return
	}

	var data bucketData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated bucket", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BucketsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/buckets/%s", state.Filesystem.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete bucket", err.Error())
	}
}

func (r *bucketsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"filesystem\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("filesystem"), parts[0])...)
}

// bucketData is the JSON:API representation of a bucket. The write-only inputs
// (key_name, key_permission, jurisdiction, cors_settings, allowed_origins) are
// preserved from configuration, not mapped back.
type bucketData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name       string  `json:"name"`
		Visibility string  `json:"visibility"`
		Endpoint   *string `json:"endpoint"`
		Status     *string `json:"status"`
		Url        *string `json:"url"`
		CreatedAt  *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *bucketData) apply(m *BucketsModel) {
	m.Id = types.StringValue(d.ID)
	m.Filesystem = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Visibility = types.StringValue(d.Attributes.Visibility)
	m.Endpoint = optionalString(d.Attributes.Endpoint)
	m.Status = optionalString(d.Attributes.Status)
	m.Url = optionalString(d.Attributes.Url)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
