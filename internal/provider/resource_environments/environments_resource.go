package resource_environments

// This file implements the environments resource. Unlike *_resource_gen.go it is
// NOT regenerated, so the CRUD logic below is safe to edit.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

var (
	_ resource.Resource                = (*environmentsResource)(nil)
	_ resource.ResourceWithConfigure   = (*environmentsResource)(nil)
	_ resource.ResourceWithImportState = (*environmentsResource)(nil)
)

// NewEnvironmentsResource returns a new environments resource.
func NewEnvironmentsResource() resource.Resource {
	return &environmentsResource{}
}

type environmentsResource struct {
	client *cloud.Client
}

func (r *environmentsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environments"
}

func (r *environmentsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *environmentsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := EnvironmentsResourceSchema(ctx)

	// application is the parent path parameter.
	providerutil.Required(s.Attributes, "application")

	// The API exposes no environment update endpoint, so every create input is
	// immutable in place.
	providerutil.RequiresReplace(s.Attributes, "branch", "name")
	providerutil.UseStateForUnknown(s.Attributes, "environment")

	s.MarkdownDescription = "Manages an environment of an application in Laravel Cloud."
	resp.Schema = s
}

func (r *environmentsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"branch": plan.Branch.ValueString(),
		"name":   plan.Name.ValueString(),
	}
	if !plan.ClusterId.IsNull() && !plan.ClusterId.IsUnknown() {
		body["cluster_id"] = plan.ClusterId.ValueString()
	}

	var data environmentData
	collection := fmt.Sprintf("/applications/%s/environments", plan.Application.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, collection, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create environment", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data environmentData
	var included []cloud.Included
	endpoint := fmt.Sprintf("/environments/%s?include=branch", state.Environment.ValueString())
	if err := r.client.DoWithIncluded(ctx, http.MethodGet, endpoint, nil, &data, &included); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read environment", err.Error())
		return
	}

	data.applyIncluded(included)
	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The API exposes no environment update body and every mutable attribute
	// forces replacement, so an update simply persists the planned values.
	var plan EnvironmentsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/environments/%s", state.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete environment", err.Error())
	}
}

func (r *environmentsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"application/environment\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment"), parts[1])...)
}

// environmentData is the JSON:API representation of an environment. The API
// has no "branch" attribute: the branch is a related resource, sideloaded with
// ?include=branch and resolved by applyIncluded.
type environmentData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
	Relationships struct {
		Branch struct {
			Data *struct {
				ID string `json:"id"`
			} `json:"data"`
		} `json:"branch"`
	} `json:"relationships"`

	branch string
}

// applyIncluded resolves the environment's branch name from the sideloaded
// "branches" resources.
func (d *environmentData) applyIncluded(included []cloud.Included) {
	for _, inc := range included {
		if inc.Type != "branches" {
			continue
		}
		if ref := d.Relationships.Branch.Data; ref != nil && ref.ID != inc.ID {
			continue
		}
		var attributes struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(inc.Attributes, &attributes); err == nil && attributes.Name != "" {
			d.branch = attributes.Name
			return
		}
	}
}

func (d *environmentData) apply(m *EnvironmentsModel) {
	m.Environment = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	// Keep the configured branch when the response does not carry it (create
	// and update responses are not sideloaded).
	if d.branch != "" {
		m.Branch = types.StringValue(d.branch)
	}
	m.ClusterId = providerutil.KnownString(m.ClusterId)
}
