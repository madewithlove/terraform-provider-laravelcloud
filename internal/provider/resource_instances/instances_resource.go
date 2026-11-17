package resource_instances

// This file implements the instances resource. Unlike *_resource_gen.go it is
// NOT regenerated, so the CRUD logic below is safe to edit.

import (
	"context"

	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	optionalBackgroundProcesses(s.Attributes)

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
	}
	setOptional(body, &plan, payloadFields(plan.Type.ValueString())...)
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
	}
	setOptional(body, &plan, payloadFields(plan.Type.ValueString())...)

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

// optionalBackgroundProcesses makes the background_processes list and its
// nested attributes optional but not computed. The API does not echo them, so
// an unset computed field would stay unknown after apply. The generators
// cannot express this, so it is applied to the generated schema here.
func optionalBackgroundProcesses(attrs map[string]schema.Attribute) {
	list, ok := attrs["background_processes"].(schema.ListNestedAttribute)
	if !ok {
		return
	}
	list.Computed = false
	nested := list.NestedObject.Attributes
	providerutil.Input(nested, "command", "processes", "type", "config")
	if config, ok := nested["config"].(schema.SingleNestedAttribute); ok {
		names := make([]string, 0, len(config.Attributes))
		for name := range config.Attributes {
			names = append(names, name)
		}
		providerutil.Input(config.Attributes, names...)
		config.Computed = false
		nested["config"] = config
	}
	attrs["background_processes"] = list
}

// payloadFields lists the optional attributes a create or update request may
// carry for an instance type. Managed queues always scale to zero when idle:
// the API rejects min_replicas for them and takes the queue timing settings
// instead, which apply to no other type.
func payloadFields(instanceType string) []string {
	fields := []string{
		"max_replicas", "uses_scheduler", "sleep_with_app",
		"scaling_cpu_threshold_percentage", "scaling_memory_threshold_percentage",
	}
	if instanceType == "managed_queue" {
		return append(fields, "visibility_timeout", "polling_interval", "shutdown_timeout")
	}
	return append(fields, "min_replicas")
}

// setOptional adds the named optional attributes to a request body when the
// plan has a known, non-null value for them.
func setOptional(body map[string]any, plan *InstancesModel, names ...string) {
	values := map[string]attr.Value{
		"max_replicas":                        plan.MaxReplicas,
		"min_replicas":                        plan.MinReplicas,
		"uses_scheduler":                      plan.UsesScheduler,
		"sleep_with_app":                      plan.SleepWithApp,
		"scaling_cpu_threshold_percentage":    plan.ScalingCpuThresholdPercentage,
		"scaling_memory_threshold_percentage": plan.ScalingMemoryThresholdPercentage,
		"visibility_timeout":                  plan.VisibilityTimeout,
		"polling_interval":                    plan.PollingInterval,
		"shutdown_timeout":                    plan.ShutdownTimeout,
	}
	for _, name := range names {
		if v := providerutil.AttrToAny(values[name]); v != nil {
			body[name] = v
		}
	}
}

// instanceData is the JSON:API representation of an instance. The nested
// background_processes block is preserved from configuration rather than mapped
// back from the response; manage existing processes with the
// background_processes resource instead.
type instanceData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name                             string  `json:"name"`
		Type                             string  `json:"type"`
		Size                             string  `json:"size"`
		ScalingType                      string  `json:"scaling_type"`
		MaxReplicas                      *int64  `json:"max_replicas"`
		MinReplicas                      *int64  `json:"min_replicas"`
		QueueStatus                      *string `json:"queue_status"`
		Paused                           *bool   `json:"paused"`
		IsDefault                        *bool   `json:"is_default"`
		VisibilityTimeout                *int64  `json:"visibility_timeout"`
		PollingInterval                  *int64  `json:"polling_interval"`
		ShutdownTimeout                  *int64  `json:"shutdown_timeout"`
		UsesScheduler                    *bool   `json:"uses_scheduler"`
		SleepWithApp                     *bool   `json:"sleep_with_app"`
		ScalingCpuThresholdPercentage    *int64  `json:"scaling_cpu_threshold_percentage"`
		ScalingMemoryThresholdPercentage *int64  `json:"scaling_memory_threshold_percentage"`
		CreatedAt                        *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *instanceData) apply(m *InstancesModel) {
	a := d.Attributes
	m.Id = types.StringValue(d.ID)
	m.Instance = types.StringValue(d.ID)
	m.Name = types.StringValue(a.Name)
	m.Type = types.StringValue(a.Type)
	m.Size = types.StringValue(a.Size)
	m.ScalingType = types.StringValue(a.ScalingType)
	m.MaxReplicas = providerutil.OptionalInt64(a.MaxReplicas)
	m.MinReplicas = providerutil.OptionalInt64(a.MinReplicas)
	m.QueueStatus = providerutil.OptionalString(a.QueueStatus)
	m.Paused = providerutil.OptionalBool(a.Paused)
	m.IsDefault = providerutil.OptionalBool(a.IsDefault)
	m.VisibilityTimeout = providerutil.OptionalInt64(a.VisibilityTimeout)
	m.PollingInterval = providerutil.OptionalInt64(a.PollingInterval)
	m.ShutdownTimeout = providerutil.OptionalInt64(a.ShutdownTimeout)
	m.UsesScheduler = providerutil.OptionalBool(a.UsesScheduler)
	m.SleepWithApp = providerutil.OptionalBool(a.SleepWithApp)
	m.ScalingCpuThresholdPercentage = providerutil.OptionalInt64(a.ScalingCpuThresholdPercentage)
	m.ScalingMemoryThresholdPercentage = providerutil.OptionalInt64(a.ScalingMemoryThresholdPercentage)
	m.CreatedAt = providerutil.OptionalString(a.CreatedAt)

	// background_processes is a create-time input that the API does not echo;
	// keep whatever the plan/state holds, and normalize unknown (e.g. a fresh
	// import) to null.
	if m.BackgroundProcesses.IsUnknown() {
		m.BackgroundProcesses = types.ListNull(BackgroundProcessesValue{}.Type(context.Background()))
	}
}
