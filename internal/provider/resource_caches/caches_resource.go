package resource_caches

// This file implements the caches resource. Unlike *_resource_gen.go it is NOT
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
	_ resource.Resource                = (*cachesResource)(nil)
	_ resource.ResourceWithConfigure   = (*cachesResource)(nil)
	_ resource.ResourceWithImportState = (*cachesResource)(nil)
)

// NewCachesResource returns a new caches resource.
func NewCachesResource() resource.Resource {
	return &cachesResource{}
}

type cachesResource struct {
	client *cloud.Client
}

func (r *cachesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_caches"
}

func (r *cachesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *cachesResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CachesResourceSchema(ctx)

	// type and region cannot be changed in place (the update endpoint excludes
	// them), so changing them forces replacement.
	providerutil.RequiresReplace(s.Attributes, "type", "region")
	providerutil.UseStateForUnknown(s.Attributes, "id", "cache")

	s.MarkdownDescription = "Manages a cache in Laravel Cloud."
	resp.Schema = s
}

func (r *cachesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CachesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"type":                 plan.Type.ValueString(),
		"name":                 plan.Name.ValueString(),
		"region":               plan.Region.ValueString(),
		"size":                 plan.Size.ValueString(),
		"auto_upgrade_enabled": plan.AutoUpgradeEnabled.ValueBool(),
		"is_public":            plan.IsPublic.ValueBool(),
	}
	if !plan.EvictionPolicy.IsNull() && !plan.EvictionPolicy.IsUnknown() {
		body["eviction_policy"] = plan.EvictionPolicy.ValueString()
	}
	if !plan.HibernationTimeout.IsNull() && !plan.HibernationTimeout.IsUnknown() {
		body["hibernation_timeout"] = plan.HibernationTimeout.ValueString()
	}
	if !plan.UsesHibernation.IsNull() && !plan.UsesHibernation.IsUnknown() {
		body["uses_hibernation"] = plan.UsesHibernation.ValueBool()
	}

	var data cacheData
	if err := r.client.Do(ctx, http.MethodPost, "/caches", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create cache", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cachesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CachesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data cacheData
	endpoint := fmt.Sprintf("/caches/%s", state.Cache.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read cache", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cachesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CachesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":                 plan.Name.ValueString(),
		"size":                 plan.Size.ValueString(),
		"auto_upgrade_enabled": plan.AutoUpgradeEnabled.ValueBool(),
		"is_public":            plan.IsPublic.ValueBool(),
	}
	if !plan.EvictionPolicy.IsNull() && !plan.EvictionPolicy.IsUnknown() {
		body["eviction_policy"] = plan.EvictionPolicy.ValueString()
	}
	if !plan.HibernationTimeout.IsNull() && !plan.HibernationTimeout.IsUnknown() {
		body["hibernation_timeout"] = plan.HibernationTimeout.ValueString()
	}
	if !plan.UsesHibernation.IsNull() && !plan.UsesHibernation.IsUnknown() {
		body["uses_hibernation"] = plan.UsesHibernation.ValueBool()
	}

	endpoint := fmt.Sprintf("/caches/%s", plan.Cache.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update cache", err.Error())
		return
	}

	var data cacheData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated cache", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cachesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CachesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/caches/%s", state.Cache.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete cache", err.Error())
	}
}

func (r *cachesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"cache\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cache"), parts[0])...)
}

// cacheData is the JSON:API representation of a cache. connection_details is a
// computed nested block that is set to a known null value in apply.
type cacheData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name               string  `json:"name"`
		Type               string  `json:"type"`
		Region             string  `json:"region"`
		Size               string  `json:"size"`
		AutoUpgradeEnabled *bool   `json:"auto_upgrade_enabled"`
		IsPublic           *bool   `json:"is_public"`
		EvictionPolicy     *string `json:"eviction_policy"`
		HibernationTimeout *string `json:"hibernation_timeout"`
		UsesHibernation    *bool   `json:"uses_hibernation"`
		Status             *string `json:"status"`
		CreatedAt          *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *cacheData) apply(m *CachesModel) {
	m.Id = types.StringValue(d.ID)
	m.Cache = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Type = types.StringValue(d.Attributes.Type)
	m.Region = types.StringValue(d.Attributes.Region)
	m.Size = types.StringValue(d.Attributes.Size)
	m.AutoUpgradeEnabled = optionalBool(d.Attributes.AutoUpgradeEnabled)
	m.IsPublic = optionalBool(d.Attributes.IsPublic)
	m.EvictionPolicy = optionalString(d.Attributes.EvictionPolicy)
	m.HibernationTimeout = optionalString(d.Attributes.HibernationTimeout)
	m.UsesHibernation = optionalBool(d.Attributes.UsesHibernation)
	m.Status = optionalString(d.Attributes.Status)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
	// connection_details is a computed nested block the API does not surface here.
	m.ConnectionDetails = NewConnectionDetailsValueNull()
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

func optionalBool(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b)
}
