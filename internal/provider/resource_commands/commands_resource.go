package resource_commands

// This file was bootstrapped by ./generator/scaffold. Unlike *_resource_gen.go
// it is NOT regenerated, so it is safe to implement the CRUD methods below.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = (*commandsResource)(nil)
	_ resource.ResourceWithConfigure   = (*commandsResource)(nil)
	_ resource.ResourceWithImportState = (*commandsResource)(nil)
)

// NewCommandsResource returns a new commands resource.
func NewCommandsResource() resource.Resource {
	return &commandsResource{}
}

type commandsResource struct {
	client *cloud.Client
}

// commandsModel mirrors the generated CommandsModel, adding the parent
// "environment" path parameter that the generator does not emit. Note the
// naming: "command" is the shell command text (the required input) while "id"
// is the resource identifier used in API paths.
type commandsModel struct {
	Environment   types.String `tfsdk:"environment"`
	Command       types.String `tfsdk:"command"`
	CreatedAt     types.String `tfsdk:"created_at"`
	ExitCode      types.Int64  `tfsdk:"exit_code"`
	FailureReason types.String `tfsdk:"failure_reason"`
	FinishedAt    types.String `tfsdk:"finished_at"`
	Id            types.String `tfsdk:"id"`
	Output        types.String `tfsdk:"output"`
	StartedAt     types.String `tfsdk:"started_at"`
	Status        types.String `tfsdk:"status"`
}

func (r *commandsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_commands"
}

func (r *commandsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *commandsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := CommandsResourceSchema(ctx)

	// The generator does not emit the parent "environment" path parameter; add
	// it as a required, replacement-forcing attribute.
	s.Attributes["environment"] = schema.StringAttribute{
		Required:            true,
		Description:         "The environment identifier",
		MarkdownDescription: "The environment identifier",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}

	// "command" is the shell command to run; it cannot be updated, so any change
	// forces replacement.
	providerutil.RequiresReplace(s.Attributes, "command")

	// The self-id is computed; keep its prior value across plans.
	providerutil.UseStateForUnknown(s.Attributes, "id")

	s.MarkdownDescription = "Runs a command on a Laravel Cloud environment."
	resp.Schema = s
}

func (r *commandsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan commandsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"command": plan.Command.ValueString(),
	}

	var data commandData
	endpoint := fmt.Sprintf("/environments/%s/commands", plan.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to run command", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *commandsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state commandsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data commandData
	endpoint := fmt.Sprintf("/commands/%s", state.Id.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read command", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never invoked with real changes: "command" forces replacement and
// every other configurable attribute is computed. It simply persists the plan.
func (r *commandsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan commandsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: a command is a one-off action with no deletion endpoint.
func (r *commandsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState supports `terraform import` using the command id.
func (r *commandsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"id\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[0])...)
}

// commandData is the JSON:API representation of a command.
type commandData struct {
	ID         string `json:"id"`
	Attributes struct {
		Command       string  `json:"command"`
		Status        string  `json:"status"`
		ExitCode      *int64  `json:"exit_code"`
		FailureReason *string `json:"failure_reason"`
		Output        *string `json:"output"`
		StartedAt     *string `json:"started_at"`
		FinishedAt    *string `json:"finished_at"`
		CreatedAt     string  `json:"created_at"`
	} `json:"attributes"`
}

// apply copies the API response onto the model. The submitted "command" text is
// echoed back, so it is safe to set here. "id" is the resource identifier.
func (d *commandData) apply(m *commandsModel) {
	m.Id = types.StringValue(d.ID)
	m.Command = types.StringValue(d.Attributes.Command)
	m.Status = types.StringValue(d.Attributes.Status)
	m.CreatedAt = types.StringValue(d.Attributes.CreatedAt)
	if d.Attributes.ExitCode != nil {
		m.ExitCode = types.Int64Value(*d.Attributes.ExitCode)
	} else {
		m.ExitCode = types.Int64Null()
	}
	if d.Attributes.FailureReason != nil {
		m.FailureReason = types.StringValue(*d.Attributes.FailureReason)
	} else {
		m.FailureReason = types.StringNull()
	}
	if d.Attributes.Output != nil {
		m.Output = types.StringValue(*d.Attributes.Output)
	} else {
		m.Output = types.StringNull()
	}
	if d.Attributes.StartedAt != nil {
		m.StartedAt = types.StringValue(*d.Attributes.StartedAt)
	} else {
		m.StartedAt = types.StringNull()
	}
	if d.Attributes.FinishedAt != nil {
		m.FinishedAt = types.StringValue(*d.Attributes.FinishedAt)
	} else {
		m.FinishedAt = types.StringNull()
	}
}
