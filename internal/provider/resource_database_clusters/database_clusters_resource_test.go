package resource_database_clusters

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestClusterType(t *testing.T) {
	tests := []struct {
		reported string
		prior    types.String
		want     string
	}{
		{"laravel_mysql", types.StringNull(), "laravel_mysql"},
		{"laravel_mysql", types.StringUnknown(), "laravel_mysql"},
		{"laravel_mysql", types.StringValue("laravel_mysql"), "laravel_mysql"},
		{"laravel_mysql", types.StringValue("laravel_mysql_84"), "laravel_mysql_84"},
		{"laravel_mysql", types.StringValue("laravel_mysql_8"), "laravel_mysql_8"},
		{"neon_serverless_postgres", types.StringValue("neon_serverless_postgres_17"), "neon_serverless_postgres_17"},
		{"aws_rds_mysql", types.StringValue("aws_rds_mysql_8"), "aws_rds_mysql_8"},
		// A different engine is real drift and must show up.
		{"neon_serverless_postgres", types.StringValue("laravel_mysql_84"), "neon_serverless_postgres"},
	}
	for _, tt := range tests {
		if got := clusterType(tt.reported, tt.prior); got != tt.want {
			t.Errorf("clusterType(%q, %s) = %q, want %q", tt.reported, tt.prior, got, tt.want)
		}
	}
}

func reportedConfig(t *testing.T) map[string]any {
	t.Helper()
	var config map[string]any
	if err := json.Unmarshal([]byte(`{
		"size": "mysql-flex-512mb", "storage": 5, "is_public": false,
		"uses_scheduled_snapshots": true, "retention_days": 7,
		"suspend_seconds": 300, "storage_autoscale_max_gb": null
	}`), &config); err != nil {
		t.Fatal(err)
	}
	return config
}

func TestClusterConfigOnImportReadsEverySetting(t *testing.T) {
	got := clusterConfig(reportedConfig(t), types.MapNull(types.StringType))
	want := types.MapValueMust(types.StringType, map[string]attr.Value{
		"size":                     types.StringValue("mysql-flex-512mb"),
		"storage":                  types.StringValue("5"),
		"is_public":                types.StringValue("false"),
		"uses_scheduled_snapshots": types.StringValue("true"),
		"retention_days":           types.StringValue("7"),
		"suspend_seconds":          types.StringValue("300"),
	})
	if !got.Equal(want) {
		t.Errorf("config = %s, want %s", got, want)
	}
}

func TestClusterConfigKeepsOnlyConfiguredSettings(t *testing.T) {
	prior := types.MapValueMust(types.StringType, map[string]attr.Value{
		"size":           types.StringValue("mysql-flex-512mb"),
		"retention_days": types.StringValue("1"),
		"not_reported":   types.StringValue("x"),
	})
	got := clusterConfig(reportedConfig(t), prior)
	want := types.MapValueMust(types.StringType, map[string]attr.Value{
		"size":           types.StringValue("mysql-flex-512mb"),
		"retention_days": types.StringValue("7"),
		"not_reported":   types.StringValue("x"),
	})
	if !got.Equal(want) {
		t.Errorf("config = %s, want %s", got, want)
	}
}

func TestConfigBodySendsTypedValues(t *testing.T) {
	config := types.MapValueMust(types.StringType, map[string]attr.Value{
		"size":      types.StringValue("mysql-flex-512mb"),
		"storage":   types.StringValue("5"),
		"is_public": types.StringValue("false"),
		"cu_min":    types.StringValue("0.25"),
	})
	body := configBody(config)
	want := map[string]any{"size": "mysql-flex-512mb", "storage": int64(5), "is_public": false, "cu_min": 0.25}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("body[%s] = %#v, want %#v", key, body[key], value)
		}
	}
	if configBody(types.MapNull(types.StringType)) != nil {
		t.Error("a null config should send nothing")
	}
}
