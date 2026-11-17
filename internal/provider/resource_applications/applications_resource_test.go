package resource_applications

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRepositoryDecodesObjectAndString(t *testing.T) {
	for name, payload := range map[string]string{
		"object (read responses)": `{"full_name": "madewithlove/sales-ai", "default_branch": "main"}`,
		"string":                  `"madewithlove/sales-ai"`,
	} {
		t.Run(name, func(t *testing.T) {
			var repository repositoryRef
			if err := json.Unmarshal([]byte(payload), &repository); err != nil {
				t.Fatal(err)
			}
			if repository.FullName != "madewithlove/sales-ai" {
				t.Errorf("FullName = %q", repository.FullName)
			}
		})
	}
}

func TestApplyMapsReadResponse(t *testing.T) {
	var data applicationData
	if err := json.Unmarshal([]byte(`{
		"id": "app-1",
		"attributes": {
			"name": "ember", "slug": "ember", "region": "eu-central-1",
			"slack_channel": null, "avatar_url": null,
			"created_at": "2026-11-10T23:29:48.000000Z",
			"repository": {"full_name": "madewithlove/sales-ai", "default_branch": "main"}
		}
	}`), &data); err != nil {
		t.Fatal(err)
	}

	model := ApplicationsModel{
		ClusterId:                 types.StringUnknown(),
		SourceControlProviderType: types.StringValue("github"),
	}
	data.apply(&model)

	if model.Repository.ValueString() != "madewithlove/sales-ai" {
		t.Errorf("repository = %s", model.Repository)
	}
	if model.Application.ValueString() != "app-1" || model.Id.ValueString() != "app-1" {
		t.Errorf("ids = %s, %s", model.Application, model.Id)
	}
	if !model.ClusterId.IsNull() {
		t.Errorf("an unknown cluster_id must resolve to null, got %s", model.ClusterId)
	}
	if model.SourceControlProviderType.ValueString() != "github" {
		t.Errorf("a configured source_control_provider_type must be kept, got %s", model.SourceControlProviderType)
	}
}
