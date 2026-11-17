package resource_environments

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/madewithlove/terraform-provider-laravelcloud/internal/cloud"
)

func TestBranchComesFromTheSideloadedBranch(t *testing.T) {
	var data environmentData
	if err := json.Unmarshal([]byte(`{
		"id": "env-1",
		"attributes": {"name": "production"},
		"relationships": {"branch": {"data": {"type": "branches", "id": "42"}}}
	}`), &data); err != nil {
		t.Fatal(err)
	}
	data.applyIncluded([]cloud.Included{
		{ID: "7", Type: "branches", Attributes: json.RawMessage(`{"name": "other"}`)},
		{ID: "42", Type: "branches", Attributes: json.RawMessage(`{"name": "main"}`)},
	})

	var model EnvironmentsModel
	data.apply(&model)
	if model.Branch.ValueString() != "main" {
		t.Errorf("branch = %s, want main", model.Branch)
	}
	if model.Environment.ValueString() != "env-1" || model.Name.ValueString() != "production" {
		t.Errorf("model = %+v", model)
	}
}

func TestBranchIsKeptWhenNotSideloaded(t *testing.T) {
	data := environmentData{ID: "env-1"}
	data.Attributes.Name = "production"

	model := EnvironmentsModel{Branch: types.StringValue("main"), ClusterId: types.StringUnknown()}
	data.apply(&model)
	if model.Branch.ValueString() != "main" {
		t.Errorf("branch = %s, want the configured main", model.Branch)
	}
	if !model.ClusterId.IsNull() {
		t.Errorf("an unknown cluster_id must resolve to null, got %s", model.ClusterId)
	}
}
