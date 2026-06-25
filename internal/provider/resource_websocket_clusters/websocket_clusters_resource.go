package resource_websocket_clusters

// This file implements the websocket_clusters resource. Unlike *_resource_gen.go
// it is NOT regenerated, so the CRUD logic below is safe to edit.

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
	_ resource.Resource                = (*websocketClustersResource)(nil)
	_ resource.ResourceWithConfigure   = (*websocketClustersResource)(nil)
	_ resource.ResourceWithImportState = (*websocketClustersResource)(nil)
)

// NewWebsocketClustersResource returns a new websocket_clusters resource.
func NewWebsocketClustersResource() resource.Resource {
	return &websocketClustersResource{}
}

type websocketClustersResource struct {
	client *cloud.Client
}

func (r *websocketClustersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_websocket_clusters"
}

func (r *websocketClustersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *websocketClustersResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := WebsocketClustersResourceSchema(ctx)

	// type and region cannot be changed in place (the update endpoint excludes
	// them), so changing them forces replacement.
	providerutil.RequiresReplace(s.Attributes, "type", "region")
	providerutil.UseStateForUnknown(s.Attributes, "id", "websocket_server")

	s.MarkdownDescription = "Manages a WebSocket cluster in Laravel Cloud."
	resp.Schema = s
}

func (r *websocketClustersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WebsocketClustersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":            plan.Name.ValueString(),
		"type":            plan.Type.ValueString(),
		"region":          plan.Region.ValueString(),
		"max_connections": plan.MaxConnections.ValueInt64(),
	}

	var data websocketClusterData
	if err := r.client.Do(ctx, http.MethodPost, "/websocket-servers", body, &data); err != nil {
		resp.Diagnostics.AddError("Unable to create WebSocket cluster", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *websocketClustersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WebsocketClustersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var data websocketClusterData
	endpoint := fmt.Sprintf("/websocket-servers/%s", state.WebsocketServer.ValueString())
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read WebSocket cluster", err.Error())
		return
	}

	data.apply(&state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *websocketClustersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WebsocketClustersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{
		"name":            plan.Name.ValueString(),
		"max_connections": plan.MaxConnections.ValueInt64(),
	}

	endpoint := fmt.Sprintf("/websocket-servers/%s", plan.WebsocketServer.ValueString())
	if err := r.client.Do(ctx, http.MethodPatch, endpoint, body, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update WebSocket cluster", err.Error())
		return
	}

	var data websocketClusterData
	if err := r.client.Do(ctx, http.MethodGet, endpoint, nil, &data); err != nil {
		resp.Diagnostics.AddError("Unable to read updated WebSocket cluster", err.Error())
		return
	}

	data.apply(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *websocketClustersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebsocketClustersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := fmt.Sprintf("/websocket-servers/%s", state.WebsocketServer.ValueString())
	if err := r.client.Do(ctx, http.MethodDelete, endpoint, nil, nil); err != nil && !cloud.NotFound(err) {
		resp.Diagnostics.AddError("Unable to delete WebSocket cluster", err.Error())
	}
}

func (r *websocketClustersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := providerutil.ParseID(req.ID, 1)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"websocket_server\": %s.", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("websocket_server"), parts[0])...)
}

// websocketClusterData is the JSON:API representation of a WebSocket cluster.
type websocketClusterData struct {
	ID         string `json:"id"`
	Attributes struct {
		Name                           string  `json:"name"`
		Type                           string  `json:"type"`
		Region                         string  `json:"region"`
		MaxConnections                 *int64  `json:"max_connections"`
		ConnectionDistributionStrategy *string `json:"connection_distribution_strategy"`
		Hostname                       *string `json:"hostname"`
		Status                         *string `json:"status"`
		CreatedAt                      *string `json:"created_at"`
	} `json:"attributes"`
}

func (d *websocketClusterData) apply(m *WebsocketClustersModel) {
	m.Id = types.StringValue(d.ID)
	m.WebsocketServer = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Attributes.Name)
	m.Type = types.StringValue(d.Attributes.Type)
	m.Region = types.StringValue(d.Attributes.Region)
	if d.Attributes.MaxConnections != nil {
		m.MaxConnections = types.Int64Value(*d.Attributes.MaxConnections)
	}
	m.ConnectionDistributionStrategy = optionalString(d.Attributes.ConnectionDistributionStrategy)
	m.Hostname = optionalString(d.Attributes.Hostname)
	m.Status = optionalString(d.Attributes.Status)
	m.CreatedAt = optionalString(d.Attributes.CreatedAt)
}

func optionalString(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}
