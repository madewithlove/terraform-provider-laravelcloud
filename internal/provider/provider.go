package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_applications"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_background_processes"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_commands"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_databases"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_deployments"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_domains"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_environments"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_filesystem_keys"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_filesystems"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/provider/resource_instances"
)

type LaravelcloudProvider struct {
	version string
}

// New returns a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &LaravelcloudProvider{version: version}
	}
}

func (p *LaravelcloudProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "laravelcloud"
	resp.Version = p.version
}

// LaravelcloudProviderModel maps the provider configuration block.
type LaravelcloudProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIToken types.String `tfsdk:"api_token"`
}

func (p *LaravelcloudProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage [Laravel Cloud](https://cloud.laravel.com) resources.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Base URL of the Laravel Cloud API. Defaults to `" + cloud.DefaultEndpoint + "`. May also be set via the `LARAVEL_CLOUD_ENDPOINT` environment variable.",
			},
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Laravel Cloud API token. May also be set via the `LARAVEL_CLOUD_TOKEN` environment variable.",
			},
		},
	}
}

func (p *LaravelcloudProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config LaravelcloudProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values take precedence over environment variables.
	endpoint := os.Getenv("LARAVEL_CLOUD_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}

	token := os.Getenv("LARAVEL_CLOUD_TOKEN")
	if !config.APIToken.IsNull() {
		token = config.APIToken.ValueString()
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing Laravel Cloud API token",
			"Set the api_token provider argument or the LARAVEL_CLOUD_TOKEN environment variable.",
		)
		return
	}

	client := cloud.New(endpoint, token)
	resp.ResourceData = client
	resp.DataSourceData = client
}

// Resources returns the resources implemented by the provider. The schemas are
// generated from the Laravel Cloud OpenAPI document; CRUD is implemented per
// resource.
func (p *LaravelcloudProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resource_applications.NewApplicationsResource,
		resource_background_processes.NewBackgroundProcessesResource,
		resource_commands.NewCommandsResource,
		resource_databases.NewDatabasesResource,
		resource_deployments.NewDeploymentsResource,
		resource_domains.NewDomainsResource,
		resource_environments.NewEnvironmentsResource,
		resource_filesystem_keys.NewFilesystemKeysResource,
		resource_filesystems.NewFilesystemsResource,
		resource_instances.NewInstancesResource,
	}
}

func (p *LaravelcloudProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
