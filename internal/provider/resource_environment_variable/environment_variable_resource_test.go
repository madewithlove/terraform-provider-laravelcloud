package resource_environment_variable

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
)

func TestVariablesAreReadFromTheEnvironment(t *testing.T) {
	var data environmentData
	if err := json.Unmarshal([]byte(`{
		"id": "env-1",
		"attributes": {
			"name": "production",
			"environment_variables": [
				{"key": "APP_ENV", "value": "production"},
				{"key": "EMPTY", "value": ""},
				{"key": "TWICE", "value": "first"},
				{"key": "TWICE", "value": "second"}
			]
		}
	}`), &data); err != nil {
		t.Fatal(err)
	}
	variables := data.Attributes.EnvironmentVariables

	cases := []struct {
		key       string
		value     string
		wantFound bool
	}{
		{"APP_ENV", "production", true},
		{"EMPTY", "", true},
		{"TWICE", "first", true},
		{"app_env", "", false},
		{"MISSING", "", false},
	}
	for _, c := range cases {
		value, found := variables.find(c.key)
		if found != c.wantFound || value != c.value {
			t.Errorf("find(%q) = %q, %v; want %q, %v", c.key, value, found, c.value, c.wantFound)
		}
	}
}

func TestAnEnvironmentWithoutVariablesHasNone(t *testing.T) {
	var data environmentData
	if err := json.Unmarshal([]byte(`{"id": "env-1", "attributes": {"environment_variables": null}}`), &data); err != nil {
		t.Fatal(err)
	}
	if _, found := data.Attributes.EnvironmentVariables.find("APP_ENV"); found {
		t.Error("found a variable in an environment without variables")
	}
}

func TestSetBodySetsOneVariable(t *testing.T) {
	assertJSON(t, setBody("CRM_WEBSITE_TOKEN", "s3cret"),
		`{"method":"set","variables":[{"key":"CRM_WEBSITE_TOKEN","value":"s3cret"}]}`)
}

func TestDeleteBodyDeletesOneKey(t *testing.T) {
	assertJSON(t, deleteBody("CRM_WEBSITE_TOKEN"), `{"keys":["CRM_WEBSITE_TOKEN"]}`)
}

func TestParseImportID(t *testing.T) {
	environment, key, err := parseImportID("env-1/CRM_WEBSITE_TOKEN")
	if err != nil || environment != "env-1" || key != "CRM_WEBSITE_TOKEN" {
		t.Errorf("parseImportID = %q, %q, %v", environment, key, err)
	}
	if got := importID(environment, key); got != "env-1/CRM_WEBSITE_TOKEN" {
		t.Errorf("importID = %q", got)
	}

	for _, id := range []string{"", "env-1", "env-1/", "/KEY", "env-1/KEY/extra", "env-1/1KEY", "env-1/MY-KEY"} {
		if _, _, err := parseImportID(id); err == nil {
			t.Errorf("parseImportID(%q) succeeded, want an error", id)
		}
	}
}

func TestLockEnvironmentIsPerEnvironment(t *testing.T) {
	unlock := lockEnvironment("env-1")
	defer unlock()

	// Another environment does not wait for env-1.
	lockEnvironment("env-2")()
}

func assertJSON(t *testing.T, body any, want string) {
	t.Helper()
	got, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

// deleteAgainst runs Delete for env-1/KEY against an API serving the given
// environment variables, whose delete endpoint answers with deleteStatus.
func deleteAgainst(t *testing.T, variables string, deleteStatus int) diag.Diagnostics {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /environments/env-1/variables/delete":
			w.WriteHeader(deleteStatus)
			_, _ = w.Write([]byte(`{"message":"The following keys do not exist: KEY"}`))
		case "GET /environments/env-1":
			_, _ = w.Write([]byte(`{"data":{"id":"env-1","attributes":{"environment_variables":` + variables + `}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	r := &environmentVariableResource{client: cloud.New(server.URL, "token")}
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema}
	if diags := state.Set(ctx, &EnvironmentVariableModel{
		Environment: types.StringValue("env-1"),
		Key:         types.StringValue("KEY"),
		Value:       types.StringValue("value"),
	}); diags.HasError() {
		t.Fatal(diags)
	}
	resp := resource.DeleteResponse{State: state}
	r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
	return resp.Diagnostics
}

func TestDeleteSucceedsWhenTheKeyIsAlreadyGone(t *testing.T) {
	if diags := deleteAgainst(t, `[{"key":"OTHER","value":"x"}]`, http.StatusUnprocessableEntity); diags.HasError() {
		t.Errorf("delete failed: %v", diags)
	}
}

func TestDeleteFailsWhenTheKeyIsStillSet(t *testing.T) {
	if diags := deleteAgainst(t, `[{"key":"KEY","value":"value"}]`, http.StatusUnprocessableEntity); !diags.HasError() {
		t.Error("delete succeeded although the key is still set")
	}
}

func TestDeleteSucceeds(t *testing.T) {
	if diags := deleteAgainst(t, `[]`, http.StatusOK); diags.HasError() {
		t.Errorf("delete failed: %v", diags)
	}
}
