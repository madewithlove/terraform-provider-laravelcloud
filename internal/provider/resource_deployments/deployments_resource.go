package resource_deployments

// The deployments resource is hand-written in full: its create operation has no
// request body (it simply triggers a deployment of the environment), which the
// OpenAPI generators cannot map, so there is no *_resource_gen.go for it.

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

var (
	_ resource.Resource                = (*deploymentsResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentsResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentsResource)(nil)
)

// NewDeploymentsResource returns a new deployments resource.
func NewDeploymentsResource() resource.Resource {
	return &deploymentsResource{}
}

type deploymentsResource struct {
	client *cloud.Client
}

// DeploymentsModel is the Terraform model for a deployment.
type DeploymentsModel struct {
	Environment     types.String `tfsdk:"environment"`
	Id              types.String `tfsdk:"id"`
	Status          types.String `tfsdk:"status"`
	BranchName      types.String `tfsdk:"branch_name"`
	CommitHash      types.String `tfsdk:"commit_hash"`
	CommitMessage   types.String `tfsdk:"commit_message"`
	BuildCommand    types.String `tfsdk:"build_command"`
	FailureReason   types.String `tfsdk:"failure_reason"`
	NodeVersion     types.String `tfsdk:"node_version"`
	PhpMajorVersion types.String `tfsdk:"php_major_version"`
	StartedAt       types.String `tfsdk:"started_at"`
	FinishedAt      types.String `tfsdk:"finished_at"`
	UsesHibernation types.Bool   `tfsdk:"uses_hibernation"`
	UsesOctane      types.Bool   `tfsdk:"uses_octane"`
}

func (r *deploymentsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployments"
}

func (r *deploymentsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *deploymentsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computed := func() schema.StringAttribute { return schema.StringAttribute{Computed: true} }
	resp.Schema = schema.Schema{
		MarkdownDescription: "Triggers a deployment of a Laravel Cloud environment.",
		Attributes: map[string]schema.Attribute{
			"environment": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The environment to deploy.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status":            computed(),
			"branch_name":       computed(),
			"commit_hash":       computed(),
			"commit_message":    computed(),
			"build_command":     computed(),
			"failure_reason":    computed(),
			"node_version":      computed(),
			"php_major_version": computed(),
			"started_at":        computed(),
			"finished_at":       computed(),
			"uses_hibernation":  schema.BoolAttribute{Computed: true},
			"uses_octane":       schema.BoolAttribute{Computed: true},
		},
	}
}

func (r *deploymentsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DeploymentsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data deploymentData
	endpoint := fmt.Sprintf("/environments/%s/deployments", plan.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create deployment", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DeploymentsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data deploymentData
	endpoint := fmt.Sprintf("/deployments/%s", state.Id.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read deployment", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never invoked with in-place changes (environment is RequiresReplace
// and all other attributes are computed).
func (r *deploymentsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DeploymentsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is a no-op: a deployment is an immutable historical record with no
// delete endpoint, so it is only removed from Terraform state.
func (r *deploymentsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *deploymentsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

type deploymentData struct {
	ID         string `json:"id"`
	Attributes struct {
		Status          string  `json:"status"`
		BranchName      *string `json:"branch_name"`
		CommitHash      *string `json:"commit_hash"`
		CommitMessage   *string `json:"commit_message"`
		BuildCommand    *string `json:"build_command"`
		FailureReason   *string `json:"failure_reason"`
		NodeVersion     *string `json:"node_version"`
		PhpMajorVersion *string `json:"php_major_version"`
		StartedAt       *string `json:"started_at"`
		FinishedAt      *string `json:"finished_at"`
		UsesHibernation *bool   `json:"uses_hibernation"`
		UsesOctane      *bool   `json:"uses_octane"`
	} `json:"attributes"`
}

func (d *deploymentData) apply(m *DeploymentsModel) {
	m.Id = types.StringValue(d.ID)
	m.Status = types.StringValue(d.Attributes.Status)
	m.BranchName = strOrNull(d.Attributes.BranchName)
	m.CommitHash = strOrNull(d.Attributes.CommitHash)
	m.CommitMessage = strOrNull(d.Attributes.CommitMessage)
	m.BuildCommand = strOrNull(d.Attributes.BuildCommand)
	m.FailureReason = strOrNull(d.Attributes.FailureReason)
	m.NodeVersion = strOrNull(d.Attributes.NodeVersion)
	m.PhpMajorVersion = strOrNull(d.Attributes.PhpMajorVersion)
	m.StartedAt = strOrNull(d.Attributes.StartedAt)
	m.FinishedAt = strOrNull(d.Attributes.FinishedAt)
	m.UsesHibernation = boolOrNull(d.Attributes.UsesHibernation)
	m.UsesOctane = boolOrNull(d.Attributes.UsesOctane)
}

func strOrNull(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

func boolOrNull(b *bool) types.Bool {
	if b == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*b)
}
