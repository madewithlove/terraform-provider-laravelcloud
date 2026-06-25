package resource_instances

// This file implements the instances resource. Unlike *_resource_gen.go it is
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
	_ resource.Resource                = (*instancesResource)(nil)
	_ resource.ResourceWithConfigure   = (*instancesResource)(nil)
	_ resource.ResourceWithImportState = (*instancesResource)(nil)
)

// NewInstancesResource returns a new instances resource.
func NewInstancesResource() resource.Resource {
	return &instancesResource{}
}

type instancesResource struct {
	client *cloud.Client
}

func (r *instancesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instances"
}

func (r *instancesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *instancesResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := InstancesResourceSchema(ctx)

	// environment is the parent path parameter.
	providerutil.Required(s.Attributes, "environment")
	// type is part of the create body but not the update body, so changing it
	// requires replacement.
	providerutil.RequiresReplace(s.Attributes, "type")
	providerutil.UseStateForUnknown(s.Attributes, "id", "instance")

	s.MarkdownDescription = "Manages an instance of an environment in Laravel Cloud."
	resp.Schema = s
}

func (r *instancesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan InstancesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":         plan.Name.ValueString(),
		"type":         plan.Type.ValueString(),
		"size":         plan.Size.ValueString(),
		"scaling_type": plan.ScalingType.ValueString(),
		"max_replicas": plan.MaxReplicas.ValueInt64(),
		"min_replicas": plan.MinReplicas.ValueInt64(),
	}
	if !plan.ScalingCpuThresholdPercentage.IsNull() && !plan.ScalingCpuThresholdPercentage.IsUnknown() {
		body["scaling_cpu_threshold_percentage"] = plan.ScalingCpuThresholdPercentage.ValueInt64()
	}
	if !plan.ScalingMemoryThresholdPercentage.IsNull() && !plan.ScalingMemoryThresholdPercentage.IsUnknown() {
		body["scaling_memory_threshold_percentage"] = plan.ScalingMemoryThresholdPercentage.ValueInt64()
	}
	if !plan.UsesScheduler.IsNull() && !plan.UsesScheduler.IsUnknown() {
		body["uses_scheduler"] = plan.UsesScheduler.ValueBool()
	}
	if bp := providerutil.AttrToAny(plan.BackgroundProcesses); bp != nil {
		body["background_processes"] = bp
	}

	var data instanceData
	collection := fmt.Sprintf("/environments/%s/instances", plan.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, collection, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create instance", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *instancesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state InstancesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data instanceData
	endpoint := fmt.Sprintf("/instances/%s", state.Instance.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read instance", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instancesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan InstancesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":         plan.Name.ValueString(),
		"size":         plan.Size.ValueString(),
		"scaling_type": plan.ScalingType.ValueString(),
		"max_replicas": plan.MaxReplicas.ValueInt64(),
		"min_replicas": plan.MinReplicas.ValueInt64(),
	}
	if !plan.ScalingCpuThresholdPercentage.IsNull() && !plan.ScalingCpuThresholdPercentage.IsUnknown() {
		body["scaling_cpu_threshold_percentage"] = plan.ScalingCpuThresholdPercentage.ValueInt64()
	}
	if !plan.ScalingMemoryThresholdPercentage.IsNull() && !plan.ScalingMemoryThresholdPercentage.IsUnknown() {
		body["scaling_memory_threshold_percentage"] = plan.ScalingMemoryThresholdPercentage.ValueInt64()
	}
	if !plan.UsesScheduler.IsNull() && !plan.UsesScheduler.IsUnknown() {
		body["uses_scheduler"] = plan.UsesScheduler.ValueBool()
	}

	endpoint := fmt.Sprintf("/instances/%s", plan.Instance.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update instance", err.Error())
		return
	}

	var data instanceData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated instance", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *instancesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state InstancesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/instances/%s", state.Instance.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete instance", err.Error())
	}
}

func (r *instancesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"environment/instance\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), parts[1])...)
}

// instanceData is the JSON:API representation of an instance. The nested
// background_processes block is preserved from configuration rather than mapped
// back from the response.
type instanceData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name                             string  `json:"name"`
		Type                             string  `json:"type"`
		Size                             string  `json:"size"`
		ScalingType                      string  `json:"scaling_type"`
		MaxReplicas                      *int64  `json:"max_replicas"`
		MinReplicas                      *int64  `json:"min_replicas"`
		ScalingCpuThresholdPercentage    *int64  `json:"scaling_cpu_threshold_percentage"`
		ScalingMemoryThresholdPercentage *int64  `json:"scaling_memory_threshold_percentage"`
		UsesScheduler                    *bool   `json:"uses_scheduler"`
		CreatedAt                        *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *instanceData) apply(m *InstancesModel) {
	m.Id = types.StringValue(d.ID)
	m.Instance = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Type = types.StringValue(d.Attributes.Type)
	m.Size = types.StringValue(d.Attributes.Size)
	m.ScalingType = types.StringValue(d.Attributes.ScalingType)
	m.MaxReplicas = optionalInt64(d.Attributes.MaxReplicas)
	m.MinReplicas = optionalInt64(d.Attributes.MinReplicas)
	m.ScalingCpuThresholdPercentage = optionalInt64(d.Attributes.ScalingCpuThresholdPercentage)
	m.ScalingMemoryThresholdPercentage = optionalInt64(d.Attributes.ScalingMemoryThresholdPercentage)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
	if d.Attributes.UsesScheduler != nil {
		m.UsesScheduler = types.BoolValue(*d.Attributes.UsesScheduler)
	} else if m.UsesScheduler.IsUnknown() {
		m.UsesScheduler = types.BoolNull()
	}

	// background_processes is configuration-driven and not reliably echoed by
	// the API; keep whatever the plan/state already holds. If it was left
	// unknown (e.g. a fresh import), normalize it to null.
	if m.BackgroundProcesses.IsUnknown() {
		m.BackgroundProcesses = types.ListNull(BackgroundProcessesValue{}.Type(context.Background()))
	}
}

func optionalInt64(i *int64) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*i)
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
