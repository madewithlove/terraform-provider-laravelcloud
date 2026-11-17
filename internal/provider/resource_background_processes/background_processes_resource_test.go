package resource_background_processes

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const workerResponse = `{
	"id": "process-1",
	"attributes": {
		"type": "worker", "processes": 1,
		"command": "php artisan queue:work database --queue=crm,default",
		"config": {
			"connection": "database", "queue": "crm,default",
			"tries": "1", "backoff": "30", "sleep": "3", "rest": "0", "timeout": "300",
			"force": null
		}
	}
}`

func decodeWorker(t *testing.T) backgroundProcessData {
	t.Helper()
	var data backgroundProcessData
	if err := json.Unmarshal([]byte(workerResponse), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReadMapsConfigOnImport(t *testing.T) {
	data := decodeWorker(t)
	model := BackgroundProcessesModel{Config: NewConfigValueNull()}
	if diags := data.apply(t.Context(), &model, true); diags.HasError() {
		t.Fatal(diags)
	}

	if model.Config.IsNull() {
		t.Fatal("config should be read on import")
	}
	if model.Config.Queue.ValueString() != "crm,default" || model.Config.Tries.ValueInt64() != 1 || model.Config.Timeout.ValueInt64() != 300 {
		t.Errorf("config = %+v", model.Config)
	}
	if !model.Config.Force.IsNull() {
		t.Errorf("force = %s, want null", model.Config.Force)
	}
}

func TestReadReportsDrift(t *testing.T) {
	data := decodeWorker(t)
	prior, diags := NewConfigValue(ConfigValue{}.AttributeTypes(t.Context()), configAttributes(map[string]attr.Value{
		"queue": types.StringValue("default"),
	}))
	if diags.HasError() {
		t.Fatal(diags)
	}
	model := BackgroundProcessesModel{Config: prior}
	data.apply(t.Context(), &model, true)
	if model.Config.Queue.ValueString() != "crm,default" {
		t.Errorf("a read must report the API's queue, got %s", model.Config.Queue)
	}
}

func TestCreateKeepsPlannedConfigAndResolvesUnknowns(t *testing.T) {
	data := decodeWorker(t)
	planned, diags := NewConfigValue(ConfigValue{}.AttributeTypes(t.Context()), configAttributes(map[string]attr.Value{
		"connection": types.StringValue("database"),
		"queue":      types.StringValue("crm,default"),
		"tries":      types.Int64Unknown(),
		"force":      types.BoolUnknown(),
	}))
	if diags.HasError() {
		t.Fatal(diags)
	}
	model := BackgroundProcessesModel{Config: planned, Command: types.StringUnknown()}
	data.apply(t.Context(), &model, false)

	if model.Config.Tries.ValueInt64() != 1 {
		t.Errorf("an unknown tries should take the API value, got %s", model.Config.Tries)
	}
	if !model.Config.Force.IsNull() {
		t.Errorf("an unknown force the API leaves null should be null, got %s", model.Config.Force)
	}
	if model.Command.IsUnknown() {
		t.Error("command must be known after apply")
	}
}

func TestLenientInt64(t *testing.T) {
	for _, payload := range []string{`5`, `"5"`} {
		var i lenientInt64
		if err := json.Unmarshal([]byte(payload), &i); err != nil || i != 5 {
			t.Errorf("decoding %s = %d, %v", payload, i, err)
		}
	}
	var i lenientInt64
	if err := json.Unmarshal([]byte(`"five"`), &i); err == nil {
		t.Error("expected an error for a non-numeric string")
	}
}

// configAttributes fills every config attribute, defaulting to null.
func configAttributes(set map[string]attr.Value) map[string]attr.Value {
	attributes := map[string]attr.Value{
		"connection": types.StringNull(), "queue": types.StringNull(),
		"tries": types.Int64Null(), "backoff": types.Int64Null(), "sleep": types.Int64Null(),
		"rest": types.Int64Null(), "timeout": types.Int64Null(), "force": types.BoolNull(),
	}
	for name, value := range set {
		attributes[name] = value
	}
	return attributes
}
