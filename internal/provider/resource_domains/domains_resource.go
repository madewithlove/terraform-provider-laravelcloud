package resource_domains

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
	_ resource.Resource                = (*domainsResource)(nil)
	_ resource.ResourceWithConfigure   = (*domainsResource)(nil)
	_ resource.ResourceWithImportState = (*domainsResource)(nil)
)

// NewDomainsResource returns a new domains resource.
func NewDomainsResource() resource.Resource {
	return &domainsResource{}
}

type domainsResource struct {
	client *cloud.Client
}

// domainsModel mirrors the generated DomainsModel, adding the parent
// "environment" path parameter that the generator does not emit.
type domainsModel struct {
	Environment        types.String `tfsdk:"environment"`
	Domain             types.String `tfsdk:"domain"`
	Name               types.String `tfsdk:"name"`
	VerificationMethod types.String `tfsdk:"verification_method"`
	Verify             types.Bool   `tfsdk:"verify"`
	WildcardEnabled    types.Bool   `tfsdk:"wildcard_enabled"`
	WwwRedirect        types.String `tfsdk:"www_redirect"`
}

func (r *domainsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (r *domainsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *domainsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DomainsResourceSchema(ctx)

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

	// name, wildcard_enabled and www_redirect are only honored at creation time;
	// verification_method is the sole updatable attribute.
	providerutil.RequiresReplace(s.Attributes, "name", "wildcard_enabled", "www_redirect")

	// "verify" triggers a separate verification action that this resource does
	// not perform; keep it as a plain optional input so it never becomes unknown.
	providerutil.Input(s.Attributes, "verify")

	// The self-id is computed; keep its prior value across plans.
	providerutil.UseStateForUnknown(s.Attributes, "domain")

	s.MarkdownDescription = "Manages a domain for a Laravel Cloud environment."
	resp.Schema = s
}

func (r *domainsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan domainsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	if !plan.VerificationMethod.IsNull() && !plan.VerificationMethod.IsUnknown() {
		body["verification_method"] = plan.VerificationMethod.ValueString()
	}
	if !plan.WildcardEnabled.IsNull() && !plan.WildcardEnabled.IsUnknown() {
		body["wildcard_enabled"] = plan.WildcardEnabled.ValueBool()
	}
	if !plan.WwwRedirect.IsNull() && !plan.WwwRedirect.IsUnknown() {
		body["www_redirect"] = plan.WwwRedirect.ValueString()
	}

	var data domainData
	endpoint := fmt.Sprintf("/environments/%s/domains", plan.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create domain", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state domainsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data domainData
	endpoint := fmt.Sprintf("/domains/%s", state.Domain.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan domainsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{}
	if !plan.VerificationMethod.IsNull() && !plan.VerificationMethod.IsUnknown() {
		body["verification_method"] = plan.VerificationMethod.ValueString()
	}

	var data domainData
	endpoint := fmt.Sprintf("/domains/%s", plan.Domain.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to update domain", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state domainsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/domains/%s", state.Domain.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete domain", err.Error())
	}
}

// ImportState supports `terraform import` using the "environment/domain"
// identifier form.
func (r *domainsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"environment/domain\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[1])...)
}

// domainData is the JSON:API representation of a domain.
type domainData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name               string  `json:"name"`
		VerificationMethod string  `json:"verification_method"`
		WildcardEnabled    *bool   `json:"wildcard_enabled"`
		WwwRedirect        *string `json:"www_redirect"`
	} `json:"attributes"`
}

// apply copies the API response onto the model. The self-id "domain" is set
// from the response's "id". Write-only inputs not echoed by the API are left
// untouched.
func (d *domainData) apply(m *domainsModel) {
	m.Domain = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.VerificationMethod = types.StringValue(d.Attributes.VerificationMethod)
	if d.Attributes.WildcardEnabled != nil {
		m.WildcardEnabled = types.BoolValue(*d.Attributes.WildcardEnabled)
	} else {
		m.WildcardEnabled = types.BoolNull()
	}
	if d.Attributes.WwwRedirect != nil {
		m.WwwRedirect = types.StringValue(*d.Attributes.WwwRedirect)
	} else {
		m.WwwRedirect = types.StringNull()
	}
}
