package resource_instances

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestApplyMapsAnAppInstance(t *testing.T) {
	var data instanceData
	if err := json.Unmarshal([]byte(`{
		"id": "inst-1",
		"attributes": {
			"name": "App", "type": "app", "size": "flex-512mb", "scaling_type": "none",
			"min_replicas": 1, "max_replicas": 1,
			"queue_status": null, "paused": null, "is_default": null,
			"visibility_timeout": null, "polling_interval": null, "shutdown_timeout": null,
			"uses_scheduler": true, "sleep_with_app": null,
			"scaling_cpu_threshold_percentage": null, "scaling_memory_threshold_percentage": null,
			"created_at": "2026-11-10T23:29:50.000000Z"
		}
	}`), &data); err != nil {
		t.Fatal(err)
	}

	model := InstancesModel{
		BackgroundProcesses: types.ListUnknown(BackgroundProcessesValue{}.Type(t.Context())),
		PollingInterval:     types.Int64Unknown(),
		Paused:              types.BoolUnknown(),
	}
	data.apply(&model)

	if model.Type.ValueString() != "app" {
		t.Errorf("type = %s", model.Type)
	}
	if !model.UsesScheduler.ValueBool() {
		t.Error("uses_scheduler should be true")
	}
	for name, value := range map[string]interface{ IsNull() bool }{
		"polling_interval":     model.PollingInterval,
		"visibility_timeout":   model.VisibilityTimeout,
		"shutdown_timeout":     model.ShutdownTimeout,
		"paused":               model.Paused,
		"is_default":           model.IsDefault,
		"queue_status":         model.QueueStatus,
		"background_processes": model.BackgroundProcesses,
	} {
		if !value.IsNull() {
			t.Errorf("%s should be null for an app instance", name)
		}
	}
}

func TestSetOptionalSkipsUnsetValues(t *testing.T) {
	plan := InstancesModel{
		MinReplicas:       types.Int64Value(1),
		MaxReplicas:       types.Int64Null(),
		PollingInterval:   types.Int64Unknown(),
		UsesScheduler:     types.BoolValue(false),
		VisibilityTimeout: types.Int64Value(30),
	}
	body := map[string]any{}
	setOptional(body, &plan, "min_replicas", "max_replicas", "polling_interval", "uses_scheduler", "visibility_timeout")

	want := map[string]any{"min_replicas": int64(1), "uses_scheduler": false, "visibility_timeout": int64(30)}
	if len(body) != len(want) {
		t.Fatalf("body = %v, want %v", body, want)
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("body[%s] = %v, want %v", key, body[key], value)
		}
	}
}

func TestPayloadFieldsDependOnType(t *testing.T) {
	plan := InstancesModel{
		MinReplicas:       types.Int64Value(0),
		PollingInterval:   types.Int64Value(20),
		VisibilityTimeout: types.Int64Value(30),
		ShutdownTimeout:   types.Int64Value(90),
	}

	queue := map[string]any{}
	setOptional(queue, &plan, payloadFields("managed_queue")...)
	if _, ok := queue["min_replicas"]; ok {
		t.Error("a managed queue must not send min_replicas")
	}
	if queue["polling_interval"] != int64(20) || queue["shutdown_timeout"] != int64(90) {
		t.Errorf("a managed queue must send its timing settings, got %v", queue)
	}

	app := map[string]any{}
	setOptional(app, &plan, payloadFields("app")...)
	if _, ok := app["polling_interval"]; ok {
		t.Error("an app instance must not send queue timing settings")
	}
	if app["min_replicas"] != int64(0) {
		t.Errorf("an app instance must send min_replicas, got %v", app)
	}
}
