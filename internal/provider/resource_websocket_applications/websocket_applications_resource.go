package resource_websocket_applications

// This file implements the websocket_applications resource. Unlike
// *_resource_gen.go it is NOT regenerated, so the CRUD logic below is safe.

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

var (
	_ resource.Resource                = (*websocketApplicationsResource)(nil)
	_ resource.ResourceWithConfigure   = (*websocketApplicationsResource)(nil)
	_ resource.ResourceWithImportState = (*websocketApplicationsResource)(nil)
)

// NewWebsocketApplicationsResource returns a new websocket_applications resource.
func NewWebsocketApplicationsResource() resource.Resource {
	return &websocketApplicationsResource{}
}

type websocketApplicationsResource struct {
	client *cloud.Client
}

func (r *websocketApplicationsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websocket_applications"
}

func (r *websocketApplicationsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *websocketApplicationsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := WebsocketApplicationsResourceSchema(ctx)

	// websocket_server is the parent path parameter.
	providerutil.Required(s.Attributes, "websocket_server")

	// All other create inputs are also updatable, so none force replacement.
	providerutil.UseStateForUnknown(s.Attributes, "websocket_application")

	s.MarkdownDescription = "Manages an application on a Laravel Cloud websocket server."
	resp.Schema = s
}

func (r *websocketApplicationsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketApplicationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data websocketApplicationData
	endpoint := fmt.Sprintf("/websocket-servers/%s/applications", plan.WebsocketServer.ValueString())
	if err := r.client.Do(ctx, http.MethodPost, endpoint, r.body(&plan), &data); err != nil {
		resp.Diagnostics.AddError("Unable to create websocket application", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *websocketApplicationsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketApplicationsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data websocketApplicationData
	endpoint := fmt.Sprintf("/websocket-applications/%s", state.WebsocketApplication.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read websocket application", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *websocketApplicationsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketApplicationsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data websocketApplicationData
	endpoint := fmt.Sprintf("/websocket-applications/%s", plan.WebsocketApplication.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, r.body(&plan), &data); err != nil {
		resp.Diagnostics.AddError("Unable to update websocket application", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *websocketApplicationsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketApplicationsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/websocket-applications/%s", state.WebsocketApplication.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete websocket application", err.Error())
	}
}

func (r *websocketApplicationsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"websocket_server/websocket_application\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("websocket_server"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("websocket_application"), parts[1])...)
}

func (r *websocketApplicationsResource) body(plan *WebsocketApplicationsModel) map[string]any {
	body := map[string]any{
		"name": plan.Name.ValueString(),
	}
	if !plan.ActivityTimeout.IsNull() && !plan.ActivityTimeout.IsUnknown() {
		body["activity_timeout"] = plan.ActivityTimeout.ValueInt64()
	}
	if !plan.PingInterval.IsNull() && !plan.PingInterval.IsUnknown() {
		body["ping_interval"] = plan.PingInterval.ValueInt64()
	}
	if v := providerutil.AttrToAny(plan.AllowedOrigins); v != nil {
		body["allowed_origins"] = v
	}
	return body
}

// websocketApplicationData is the JSON:API representation of a websocket
// application. There is no top-level "id" model field; the self-id is
// websocket_application.
type websocketApplicationData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name            string   `json:"name"`
		ActivityTimeout *int64   `json:"activity_timeout"`
		PingInterval    *int64   `json:"ping_interval"`
		AllowedOrigins  []string `json:"allowed_origins"`
	} `json:"attributes"`
}

func (d *websocketApplicationData) apply(m *WebsocketApplicationsModel) {
	m.WebsocketApplication = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)

	if d.Attributes.ActivityTimeout != nil {
		m.ActivityTimeout = types.Int64Value(*d.Attributes.ActivityTimeout)
	} else {
		m.ActivityTimeout = types.Int64Null()
	}
	if d.Attributes.PingInterval != nil {
		m.PingInterval = types.Int64Value(*d.Attributes.PingInterval)
	} else {
		m.PingInterval = types.Int64Null()
	}

	if d.Attributes.AllowedOrigins != nil {
		elems := make([]attr.Value, 0, len(d.Attributes.AllowedOrigins))
		for _, o := range d.Attributes.AllowedOrigins {
			elems = append(elems, types.StringValue(o))
		}
		m.AllowedOrigins = types.ListValueMust(types.StringType, elems)
	} else {
		m.AllowedOrigins = types.ListNull(types.StringType)
	}
}
