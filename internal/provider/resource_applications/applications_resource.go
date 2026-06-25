package resource_applications

// This file implements the applications resource. Unlike *_resource_gen.go it is
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
	_ resource.Resource                = (*applicationsResource)(nil)
	_ resource.ResourceWithConfigure   = (*applicationsResource)(nil)
	_ resource.ResourceWithImportState = (*applicationsResource)(nil)
)

// NewApplicationsResource returns a new applications resource.
func NewApplicationsResource() resource.Resource {
	return &applicationsResource{}
}

type applicationsResource struct {
	client *cloud.Client
}

func (r *applicationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_applications"
}

func (r *applicationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *applicationsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := ApplicationsResourceSchema(ctx)

	// The API exposes no update endpoint for applications, so every create input
	// is immutable in place.
	providerutil.RequiresReplace(s.Attributes, "name", "region", "repository")
	providerutil.UseStateForUnknown(s.Attributes, "id", "application")

	s.MarkdownDescription = "Manages an application in Laravel Cloud."
	resp.Schema = s
}

func (r *applicationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ApplicationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"region":     plan.Region.ValueString(),
		"repository": plan.Repository.ValueString(),
	}

	var data applicationData
	if err := r.client.Do(ctx, http.MethodPost, "/applications", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create application", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *applicationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ApplicationsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data applicationData
	endpoint := fmt.Sprintf("/applications/%s", state.Application.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read application", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The API exposes no application update endpoint and every mutable attribute
	// forces replacement, so an update simply persists the planned values.
	var plan ApplicationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *applicationsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// The API exposes no application delete endpoint; removing the resource from
	// Terraform state (handled by the framework) is the best we can do.
}

func (r *applicationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"application\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application"), parts[0])...)
}

// applicationData is the JSON:API representation of an application.
type applicationData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name         string  `json:"name"`
		Region       string  `json:"region"`
		Repository   string  `json:"repository"`
		Slug         *string `json:"slug"`
		SlackChannel *string `json:"slack_channel"`
		CreatedAt    *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *applicationData) apply(m *ApplicationsModel) {
	m.Id = types.StringValue(d.ID)
	m.Application = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Region = types.StringValue(d.Attributes.Region)
	m.Repository = types.StringValue(d.Attributes.Repository)
	m.Slug = optionalString(d.Attributes.Slug)
	m.SlackChannel = optionalString(d.Attributes.SlackChannel)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
