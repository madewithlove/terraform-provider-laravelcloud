package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestApplySchemaFixesToPinnedSpec(t *testing.T) {
	raw, err := os.ReadFile("../docs.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	components, _ := doc["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)

	if err := applySchemaFixes(schemas); err != nil {
		t.Fatalf("every fix must still match the pinned spec: %v", err)
	}

	databaseTypes, _ := mustLookup(t, schemas, "DatabaseType")["enum"].([]any)
	for _, want := range []string{"laravel_mysql_84", "laravel_mysql", "neon_serverless_postgres"} {
		if !containsValue(databaseTypes, want) {
			t.Errorf("DatabaseType enum is missing %q: %v", want, databaseTypes)
		}
	}

	instance := mustLookup(t, schemas, "CreateInstanceRequest")
	instanceTypes, _ := mustLookup(t, schemas, "CreateInstanceRequest.type")["enum"].([]any)
	for _, want := range []string{"service", "managed_queue", "app", "queue"} {
		if !containsValue(instanceTypes, want) {
			t.Errorf("CreateInstanceRequest.type enum is missing %q: %v", want, instanceTypes)
		}
	}
	if got, want := instance["required"], []any{"name", "type", "size", "scaling_type"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CreateInstanceRequest.required = %v, want %v", got, want)
	}
	shutdown := mustLookup(t, schemas, "CreateInstanceRequest.shutdown_timeout")
	if _, ok := shutdown["maximum"]; ok {
		t.Errorf("shutdown_timeout keeps its impossible bounds: %v", shutdown)
	}

	if dcv := mustLookup(t, schemas, "DomainResource.attributes.dns_records.dcv"); dcv["type"] != "object" {
		t.Errorf("dns_records.dcv should be an object, got %v", dcv)
	}
}

func TestApplySchemaFixesIsIdempotent(t *testing.T) {
	schemas := map[string]any{
		"DatabaseType": map[string]any{"type": "string", "enum": []any{"laravel_mysql"}},
	}
	saved := enumExtensions
	t.Cleanup(func() { enumExtensions = saved })
	enumExtensions = map[string][]string{"DatabaseType": {"laravel_mysql", "neon_serverless_postgres"}}
	savedOptional, savedOverrides := optionalFields, propertyOverrides
	t.Cleanup(func() { optionalFields, propertyOverrides = savedOptional, savedOverrides })
	optionalFields, propertyOverrides = nil, nil

	for i := 0; i < 2; i++ {
		if err := applySchemaFixes(schemas); err != nil {
			t.Fatal(err)
		}
	}
	got := mustLookup(t, schemas, "DatabaseType")["enum"]
	if want := []any{"laravel_mysql", "neon_serverless_postgres"}; !reflect.DeepEqual(got, want) {
		t.Errorf("enum = %v, want %v", got, want)
	}
}

func TestApplySchemaFixesReportsStaleEntries(t *testing.T) {
	savedEnums, savedOptional, savedOverrides := enumExtensions, optionalFields, propertyOverrides
	t.Cleanup(func() { enumExtensions, optionalFields, propertyOverrides = savedEnums, savedOptional, savedOverrides })
	enumExtensions, optionalFields = nil, nil
	propertyOverrides = map[string]map[string]any{"DomainResource.attributes.gone": {"type": "string"}}

	schemas := map[string]any{
		"DomainResource": map[string]any{"properties": map[string]any{
			"attributes": map[string]any{"properties": map[string]any{}},
		}},
	}
	err := applySchemaFixes(schemas)
	if err == nil || !strings.Contains(err.Error(), "DomainResource.attributes.gone") {
		t.Fatalf("expected an error naming the stale entry, got %v", err)
	}
}

func mustLookup(t *testing.T, schemas map[string]any, key string) map[string]any {
	t.Helper()
	schema, err := lookupSchema(schemas, key)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
