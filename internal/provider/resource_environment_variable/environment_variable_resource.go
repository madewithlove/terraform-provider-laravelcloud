// Package resource_environment_variable implements the environment variable
// resource. The API has no CRUD resource for a single variable (variables are
// set and deleted in bulk on their environment), so the code generator cannot
// map it and this resource is written by hand: there is no *_resource_gen.go.
package resource_environment_variable

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/providerutil"
)

var (
	_ resource.Resource                = (*environmentVariableResource)(nil)
	_ resource.ResourceWithConfigure   = (*environmentVariableResource)(nil)
	_ resource.ResourceWithImportState = (*environmentVariableResource)(nil)
)

// keyPattern is the variable name pattern the API accepts.
var keyPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// NewEnvironmentVariableResource returns a new environment variable resource.
func NewEnvironmentVariableResource() resource.Resource {
	return &environmentVariableResource{}
}

type environmentVariableResource struct {
	client *cloud.Client
}

// EnvironmentVariableModel maps the resource schema.
type EnvironmentVariableModel struct {
	Environment types.String `tfsdk:"environment"`
	Key         types.String `tfsdk:"key"`
	Value       types.String `tfsdk:"value"`
}

func (r *environmentVariableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment_variable"
}

func (r *environmentVariableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerutil.Client(req, resp)
}

func (r *environmentVariableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages one environment variable of an environment in Laravel Cloud.\n\n" +
			"Each resource manages a single key and leaves the environment's other variables alone, " +
			"so variables set in the dashboard or with the `cloud` CLI can live next to the ones Terraform manages.\n\n" +
			"A new or changed value is stored right away but only reaches the application with the next deployment: " +
			"Laravel Cloud does not redeploy the environment when a variable changes " +
			"([docs](https://cloud.laravel.com/docs/environments)), so deploy the environment afterwards.\n\n" +
			"Creating a variable whose key is already set on the environment fails instead of overwriting it; " +
			"import the variable to manage it.",
		Attributes: map[string]schema.Attribute{
			"environment": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the environment the variable belongs to.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the variable, such as `APP_KEY`: a letter or underscore followed by letters, digits and underscores.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.RegexMatches(keyPattern, "must start with a letter or underscore and contain only letters, digits and underscores"),
				},
			},
			"value": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "The value of the variable.",
			},
		},
	}
}

func (r *environmentVariableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan EnvironmentVariableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	environment, key := plan.Environment.ValueString(), plan.Key.ValueString()

	unlock := lockEnvironment(environment)
	defer unlock()

	variables, err := r.variables(ctx, environment)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read environment variables", err.Error())
		return
	}
	if _, exists := variables.find(key); exists {
		resp.Diagnostics.AddAttributeError(path.Root("key"), "Environment variable already exists",
			fmt.Sprintf("%s is already set on environment %s. Import it to manage it with Terraform, "+
				"for example with an import block whose id is %q.", key, environment, importID(environment, key)))
		return
	}

	if err := r.set(ctx, environment, key, plan.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to set environment variable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentVariableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state EnvironmentVariableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	variables, err := r.variables(ctx, state.Environment.ValueString())
	if err != nil {
		if cloud.NotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read environment variables", err.Error())
		return
	}

	value, exists := variables.find(state.Key.ValueString())
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Value = types.StringValue(value)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentVariableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EnvironmentVariableModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	environment := plan.Environment.ValueString()

	unlock := lockEnvironment(environment)
	defer unlock()

	if err := r.set(ctx, environment, plan.Key.ValueString(), plan.Value.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to set environment variable", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *environmentVariableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state EnvironmentVariableModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	environment, key := state.Environment.ValueString(), state.Key.ValueString()

	unlock := lockEnvironment(environment)
	defer unlock()

	endpoint := fmt.Sprintf("/environments/%s/variables/delete", environment)
	err := r.client.Do(ctx, http.MethodPost, endpoint, deleteBody(key), nil)
	if err == nil || cloud.NotFound(err) {
		return
	}

	// The delete is atomic and fails when a key is missing. A variable that
	// was already removed outside Terraform is as good as deleted.
	variables, readErr := r.variables(ctx, environment)
	if readErr == nil {
		if _, exists := variables.find(key); !exists {
			return
		}
	} else if cloud.NotFound(readErr) {
		return
	}
	resp.Diagnostics.AddError("Unable to delete environment variable", err.Error())
}

func (r *environmentVariableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	environment, key, err := parseImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment"), environment)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("key"), key)...)
}

// variables reads the environment's variables.
func (r *environmentVariableResource) variables(ctx context.Context, environment string) (variableList, error) {
	var data environmentData
	if err := r.client.Do(ctx, http.MethodGet, fmt.Sprintf("/environments/%s", environment), nil, &data); err != nil {
		return nil, err
	}
	return data.Attributes.EnvironmentVariables, nil
}

// set sets one variable, adding it or replacing the value of the same key.
func (r *environmentVariableResource) set(ctx context.Context, environment, key, value string) error {
	endpoint := fmt.Sprintf("/environments/%s/variables", environment)
	return r.client.Do(ctx, http.MethodPost, endpoint, setBody(key, value), nil)
}

// environmentData is the part of the JSON:API environment the resource reads.
// The API returns the variables' values unmasked.
type environmentData struct {
	Attributes struct {
		EnvironmentVariables variableList `json:"environment_variables"`
	} `json:"attributes"`
}

type variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type variableList []variable

// find returns the value of key. A key set more than once (the API's "append"
// method allows duplicates) reads as its first value.
func (l variableList) find(key string) (string, bool) {
	for _, v := range l {
		if v.Key == key {
			return v.Value, true
		}
	}
	return "", false
}

// setBody is the request that sets one variable: "set" replaces the value of a
// matching key and adds the key otherwise, where "append" would add a
// duplicate.
func setBody(key, value string) map[string]any {
	return map[string]any{
		"method":    "set",
		"variables": []variable{{Key: key, Value: value}},
	}
}

// deleteBody is the request that deletes one variable.
func deleteBody(key string) map[string]any {
	return map[string]any{"keys": []string{key}}
}

func importID(environment, key string) string {
	return environment + "/" + key
}

// parseImportID splits an "environment/KEY" import ID.
func parseImportID(id string) (environment, key string, err error) {
	parts, err := providerutil.ParseID(id, 2)
	if err != nil || strings.TrimSpace(parts[0]) == "" || !keyPattern.MatchString(parts[1]) {
		return "", "", fmt.Errorf("expected an import ID in the form \"environment-id/KEY\", got %q", id)
	}
	return parts[0], parts[1], nil
}

// environmentLocks serializes the changes to an environment's variables. The
// API stores them as one list per environment, and Terraform applies resources
// in parallel.
var (
	environmentLocksMu sync.Mutex
	environmentLocks   = map[string]*sync.Mutex{}
)

func lockEnvironment(environment string) (unlock func()) {
	environmentLocksMu.Lock()
	mutex, ok := environmentLocks[environment]
	if !ok {
		mutex = &sync.Mutex{}
		environmentLocks[environment] = mutex
	}
	environmentLocksMu.Unlock()

	mutex.Lock()
	return mutex.Unlock
}
