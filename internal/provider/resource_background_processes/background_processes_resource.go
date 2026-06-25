package resource_background_processes

// This file implements the background_processes resource. Unlike
// *_resource_gen.go it is NOT regenerated, so the CRUD logic below is safe.

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
	_ resource.Resource                = (*backgroundProcessesResource)(nil)
	_ resource.ResourceWithConfigure   = (*backgroundProcessesResource)(nil)
	_ resource.ResourceWithImportState = (*backgroundProcessesResource)(nil)
)

// NewBackgroundProcessesResource returns a new background_processes resource.
func NewBackgroundProcessesResource() resource.Resource {
	return &backgroundProcessesResource{}
}

type backgroundProcessesResource struct {
	client *cloud.Client
}

func (r *backgroundProcessesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_background_processes"
}

func (r *backgroundProcessesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *backgroundProcessesResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := BackgroundProcessesResourceSchema(ctx)

	providerutil.Required(s.Attributes, "instance")
	providerutil.Input(s.Attributes, "config")
	providerutil.UseStateForUnknown(s.Attributes, "background_process")

	s.MarkdownDescription = "Manages a background process on a Laravel Cloud instance."
	resp.Schema = s
}

func (r *backgroundProcessesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan BackgroundProcessesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/instances/%s/background-processes", plan.Instance.ValueString())
	var data backgroundProcessData
	if err := r.client.Do(ctx, http.MethodPost, endpoint, r.body(&plan), &data); err != nil {
		resp.Diagnostics.AddError("Unable to create background process", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *backgroundProcessesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state BackgroundProcessesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data backgroundProcessData
	endpoint := fmt.Sprintf("/background-processes/%s", state.BackgroundProcess.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read background process", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *backgroundProcessesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan BackgroundProcessesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/background-processes/%s", plan.BackgroundProcess.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, r.body(&plan), nil); err != nil {
		resp.Diagnostics.AddError("Unable to update background process", err.Error())
		return
	}

	var data backgroundProcessData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated background process", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *backgroundProcessesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state BackgroundProcessesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/background-processes/%s", state.BackgroundProcess.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete background process", err.Error())
	}
}

func (r *backgroundProcessesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"instance/background_process\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("instance"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("background_process"), parts[1])...)
}

func (r *backgroundProcessesResource) body(plan *BackgroundProcessesModel) map[string]any {
	body := map[string]any{
		"type":      plan.Type.ValueString(),
		"processes": plan.Processes.ValueInt64(),
	}
	if !plan.Command.IsNull() && !plan.Command.IsUnknown() {
		body["command"] = plan.Command.ValueString()
	}
	if v := providerutil.AttrToAny(plan.Config); v != nil {
		body["config"] = v
	}
	return body
}

// backgroundProcessData is the JSON:API representation. The "config" block is a
// write-only input preserved from configuration, not mapped back.
type backgroundProcessData struct {
	ID         string `json:"id"`
	Attributes struct {
		Command   *string `json:"command"`
		Processes *int64  `json:"processes"`
		Type      string  `json:"type"`
	} `json:"attributes"`
}

func (d *backgroundProcessData) apply(m *BackgroundProcessesModel) {
	m.BackgroundProcess = types.StringValue(d.ID)
	m.Type = types.StringValue(d.Attributes.Type)
	if d.Attributes.Command != nil {
		m.Command = types.StringValue(*d.Attributes.Command)
	}
	if d.Attributes.Processes != nil {
		m.Processes = types.Int64Value(*d.Attributes.Processes)
	}
}
