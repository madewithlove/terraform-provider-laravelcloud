package resource_domains

// This file was bootstrapped by ./generator/scaffold. Unlike *_resource_gen.go
// it is NOT regenerated, so it is safe to implement the CRUD methods below.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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

func (r *domainsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (r *domainsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *domainsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := DomainsResourceSchema(ctx)

	// environment is the parent path parameter.
	providerutil.Required(s.Attributes, "environment")

	// These are only honored at creation time; verification_method is the sole
	// updatable attribute.
	providerutil.RequiresReplace(s.Attributes, "name", "cloudflare_strategy", "wildcard_enabled", "www_redirect")

	// "verify" triggers a separate verification action that this resource does
	// not perform; keep it as a plain optional input so it never becomes unknown.
	providerutil.Input(s.Attributes, "verify")

	// The create-only attributes are optional and computed: without
	// UseStateForUnknown, leaving one unset would plan it as unknown on any
	// change and replace the domain.
	providerutil.UseStateForUnknown(s.Attributes, "id", "domain", "created_at",
		"cloudflare_strategy", "wildcard_enabled", "www_redirect", "allow_downtime", "verification_method")

	s.MarkdownDescription = "Manages a domain for a Laravel Cloud environment."
	resp.Schema = s
}

func (r *domainsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	for key, value := range map[string]attr.Value{
		"cloudflare_strategy": plan.CloudflareStrategy,
		"allow_downtime":      plan.AllowDowntime,
		"verification_method": plan.VerificationMethod,
		"wildcard_enabled":    plan.WildcardEnabled,
		"www_redirect":        plan.WwwRedirect,
	} {
		if v := providerutil.AttrToAny(value); v != nil {
			body[key] = v
		}
	}

	var data domainData
	endpoint := fmt.Sprintf("/environments/%s/domains", plan.Environment.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create domain", err.Error())
		return
	}

	resp.Diagnostics.Append(data.apply(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainsModel
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

	resp.Diagnostics.Append(data.apply(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *domainsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DomainsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/domains/%s", plan.Domain.ValueString())
	var data domainData
	if !plan.VerificationMethod.IsNull() && !plan.VerificationMethod.IsUnknown() {
		body := map[string]any{"verification_method": plan.VerificationMethod.ValueString()}
		if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, &data); err != nil {
			resp.Diagnostics.AddError("Unable to update domain", err.Error())
			return
		}
	} else if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		// Nothing the API can update changed (e.g. only "verify" or
		// "allow_downtime" did); refresh the computed attributes.
		resp.Diagnostics.AddError("Unable to read domain", err.Error())
		return
	}

	resp.Diagnostics.Append(data.apply(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *domainsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainsModel
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

// dnsRecord is a DNS record Laravel Cloud asks for.
type dnsRecord struct {
	Type  *string `json:"type"`
	Name  *string `json:"name"`
	Value *string `json:"value"`
}

// domainData is the JSON:API representation of a domain.
type domainData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name               string  `json:"name"`
		HostnameStatus     *string `json:"hostname_status"`
		SslStatus          *string `json:"ssl_status"`
		OriginStatus       *string `json:"origin_status"`
		Redirect           *string `json:"redirect"`
		CloudflareStrategy *string `json:"cloudflare_strategy"`
		Downtime           *bool   `json:"downtime"`
		WildcardEnabled    *bool   `json:"wildcard_enabled"`
		ActionRequired     *string `json:"action_required"`
		LastVerifiedAt     *string `json:"last_verified_at"`
		CreatedAt          *string `json:"created_at"`
		DnsRecords         *struct {
			Ssl             []dnsRecord `json:"ssl"`
			PreVerification *dnsRecord  `json:"pre_verification"`
			Origin          *dnsRecord  `json:"origin"`
			OriginCname     *dnsRecord  `json:"origin_cname"`
			Dcv             *dnsRecord  `json:"dcv"`
		} `json:"dns_records"`
	} `json:"attributes"`
}

// apply copies the API response onto the model. Create-time inputs the API
// does not echo (allow_downtime, verification_method, verify) keep their
// planned or prior value.
func (d *domainData) apply(ctx context.Context, m *DomainsModel) diag.Diagnostics {
	a := d.Attributes
	m.Id = types.StringValue(d.ID)
	m.Domain = types.StringValue(d.ID)
	m.Name = types.StringValue(a.Name)
	m.HostnameStatus = providerutil.OptionalString(a.HostnameStatus)
	m.SslStatus = providerutil.OptionalString(a.SslStatus)
	m.OriginStatus = providerutil.OptionalString(a.OriginStatus)
	m.Redirect = providerutil.OptionalString(a.Redirect)
	// The response reports the configured www_redirect as "redirect".
	m.WwwRedirect = providerutil.OptionalString(a.Redirect)
	m.CloudflareStrategy = providerutil.OptionalString(a.CloudflareStrategy)
	m.Downtime = providerutil.OptionalBool(a.Downtime)
	m.WildcardEnabled = providerutil.OptionalBool(a.WildcardEnabled)
	m.ActionRequired = providerutil.OptionalString(a.ActionRequired)
	m.LastVerifiedAt = providerutil.OptionalString(a.LastVerifiedAt)
	m.CreatedAt = providerutil.OptionalString(a.CreatedAt)

	m.AllowDowntime = providerutil.KnownBool(m.AllowDowntime)
	m.VerificationMethod = providerutil.KnownString(m.VerificationMethod)
	m.Verify = providerutil.KnownBool(m.Verify)

	if a.DnsRecords == nil {
		m.DnsRecords = NewDnsRecordsValueNull()
		return nil
	}

	var diags diag.Diagnostics
	ssl := make([]attr.Value, 0, len(a.DnsRecords.Ssl))
	for _, record := range a.DnsRecords.Ssl {
		value, valueDiags := NewSslValue(SslValue{}.AttributeTypes(ctx), record.attributes())
		diags.Append(valueDiags...)
		ssl = append(ssl, value)
	}
	sslList, valueDiags := types.ListValue(SslValue{}.Type(ctx), ssl)
	diags.Append(valueDiags...)

	records, valueDiags := NewDnsRecordsValue(DnsRecordsValue{}.AttributeTypes(ctx), map[string]attr.Value{
		"ssl":              sslList,
		"pre_verification": a.DnsRecords.PreVerification.object(DcvValue{}.AttributeTypes(ctx)),
		"origin":           a.DnsRecords.Origin.object(DcvValue{}.AttributeTypes(ctx)),
		"origin_cname":     a.DnsRecords.OriginCname.object(DcvValue{}.AttributeTypes(ctx)),
		"dcv":              a.DnsRecords.Dcv.object(DcvValue{}.AttributeTypes(ctx)),
	})
	diags.Append(valueDiags...)
	m.DnsRecords = records
	return diags
}

func (r *dnsRecord) attributes() map[string]attr.Value {
	return map[string]attr.Value{
		"type":  providerutil.OptionalString(r.Type),
		"name":  providerutil.OptionalString(r.Name),
		"value": providerutil.OptionalString(r.Value),
	}
}

// object converts a record into a plain object value, or null when absent.
// The dcv, origin, origin_cname and pre_verification objects share one shape.
func (r *dnsRecord) object(attributeTypes map[string]attr.Type) types.Object {
	if r == nil {
		return types.ObjectNull(attributeTypes)
	}
	return types.ObjectValueMust(attributeTypes, r.attributes())
}
