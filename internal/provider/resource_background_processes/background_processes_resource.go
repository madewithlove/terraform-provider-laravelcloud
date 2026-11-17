package resource_background_processes

// This file implements the background_processes resource. Unlike
// *_resource_gen.go it is NOT regenerated, so the CRUD logic below is safe.

import (
	"context"
	"encoding/json"
	"strconv"

	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

	resp.Diagnostics.Append(data.apply(ctx, &plan, false)...)
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

	resp.Diagnostics.Append(data.apply(ctx, &state, true)...)
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

	resp.Diagnostics.Append(data.apply(ctx, &plan, false)...)
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

// backgroundProcessData is the JSON:API representation of a background process.
type backgroundProcessData struct {
	ID         string `json:"id"`
	Attributes struct {
		Command   *string        `json:"command"`
		Processes *int64         `json:"processes"`
		Type      string         `json:"type"`
		Config    *processConfig `json:"config"`
	} `json:"attributes"`
}

// processConfig is a worker's queue settings as the API returns them. The API
// sends the numeric settings as strings ("tries": "1"), so they are decoded
// leniently.
type processConfig struct {
	Connection *string       `json:"connection"`
	Queue      *string       `json:"queue"`
	Tries      *lenientInt64 `json:"tries"`
	Backoff    *lenientInt64 `json:"backoff"`
	Sleep      *lenientInt64 `json:"sleep"`
	Rest       *lenientInt64 `json:"rest"`
	Timeout    *lenientInt64 `json:"timeout"`
	Force      *bool         `json:"force"`
}

// lenientInt64 decodes an integer sent either as a JSON number or a string.
type lenientInt64 int64

func (i *lenientInt64) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*i = lenientInt64(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("decoding integer: %w", err)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("decoding integer %q: %w", s, err)
	}
	*i = lenientInt64(n)
	return nil
}

func (c *processConfig) empty() bool {
	return c == nil || (c.Connection == nil && c.Queue == nil && c.Tries == nil && c.Backoff == nil &&
		c.Sleep == nil && c.Rest == nil && c.Timeout == nil && c.Force == nil)
}

// apply copies the API response onto the model. On a read (fromRead), the
// API's config settings win so drift is detected; after create or update, the
// planned settings are kept and only the ones left unknown are filled in.
func (d *backgroundProcessData) apply(ctx context.Context, m *BackgroundProcessesModel, fromRead bool) diag.Diagnostics {
	m.BackgroundProcess = types.StringValue(d.ID)
	m.Type = types.StringValue(d.Attributes.Type)
	if d.Attributes.Command != nil {
		m.Command = types.StringValue(*d.Attributes.Command)
	} else {
		m.Command = providerutil.KnownString(m.Command)
	}
	if d.Attributes.Processes != nil {
		m.Processes = types.Int64Value(*d.Attributes.Processes)
	}

	config := d.Attributes.Config
	if config.empty() {
		if fromRead || m.Config.IsUnknown() {
			m.Config = NewConfigValueNull()
		}
		return nil
	}
	if !fromRead && m.Config.IsNull() {
		// Not configured: keep it null rather than adopting the API defaults,
		// so the state matches the plan.
		return nil
	}

	prior := m.Config
	known := !prior.IsNull() && !prior.IsUnknown()
	pick := func(apiValue attr.Value, priorValue attr.Value) attr.Value {
		if !fromRead && known && priorValue != nil && !priorValue.IsUnknown() {
			return priorValue
		}
		return apiValue
	}
	var priorAttrs map[string]attr.Value
	if known {
		priorAttrs = map[string]attr.Value{
			"connection": prior.Connection, "queue": prior.Queue, "tries": prior.Tries,
			"backoff": prior.Backoff, "sleep": prior.Sleep, "rest": prior.Rest,
			"timeout": prior.Timeout, "force": prior.Force,
		}
	}
	apiAttrs := map[string]attr.Value{
		"connection": providerutil.OptionalString(config.Connection),
		"queue":      providerutil.OptionalString(config.Queue),
		"tries":      config.Tries.value(),
		"backoff":    config.Backoff.value(),
		"sleep":      config.Sleep.value(),
		"rest":       config.Rest.value(),
		"timeout":    config.Timeout.value(),
		"force":      providerutil.OptionalBool(config.Force),
	}
	attributes := make(map[string]attr.Value, len(apiAttrs))
	for name, value := range apiAttrs {
		attributes[name] = pick(value, priorAttrs[name])
	}

	value, diags := NewConfigValue(ConfigValue{}.AttributeTypes(ctx), attributes)
	m.Config = value
	return diags
}

func (i *lenientInt64) value() types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*i))
}
