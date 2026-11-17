package main

import (
	"fmt"
	"strings"
)

// The tables in this file correct places where the upstream OpenAPI document
// disagrees with what the live API returns. Each one exists because importing
// a real resource failed or drifted; extend them rather than hand-editing the
// generated *_resource_gen.go files.

// enumExtensions adds values to an enum. The key is a component schema name
// (for a named enum such as "DatabaseType") or "Schema.property" for an enum
// declared inline on a property. Values already present are not duplicated.
var enumExtensions = map[string][]string{
	// The API reports a cluster's engine family without the version suffix the
	// create request uses ("laravel_mysql", not "laravel_mysql_84"), and newer
	// API versions accept the unversioned names on create too.
	"DatabaseType": {"laravel_mysql", "aws_rds_mysql", "aws_rds_postgres", "neon_serverless_postgres"},
	// Every environment gets an "app" instance, and older environments may run
	// a "queue" instance. The create request only lists the types it can create,
	// so an existing app instance could not be imported.
	"CreateInstanceRequest.type": {"app", "queue"},
}

// optionalFields removes properties from a schema's "required" list. The key is
// a component schema name.
var optionalFields = map[string][]string{
	// These only apply to managed queues and are null for app and service
	// instances; the current upstream spec no longer requires them either.
	"CreateInstanceRequest": {"min_replicas", "visibility_timeout", "polling_interval", "shutdown_timeout"},
}

// propertyOverrides replaces a property's schema. The key is a component schema
// name followed by the property path, with "attributes" for the JSON:API
// attributes member (e.g. "DomainResource.attributes.dns_records.dcv").
var propertyOverrides = map[string]map[string]any{
	// The spec types these DNS records as strings; the API returns
	// {type, name, value} objects (or null).
	"DomainResource.attributes.dns_records.dcv":              dnsRecordSchema(),
	"DomainResource.attributes.dns_records.origin":           dnsRecordSchema(),
	"DomainResource.attributes.dns_records.origin_cname":     dnsRecordSchema(),
	"DomainResource.attributes.dns_records.pre_verification": dnsRecordSchema(),
	// The pinned spec bounds this to minimum 1, maximum 0, which rejects every
	// value. The current upstream spec drops the bounds; so do we.
	"CreateInstanceRequest.shutdown_timeout": {"type": "integer"},
}

func dnsRecordSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":  map[string]any{"type": "string"},
			"name":  map[string]any{"type": "string"},
			"value": map[string]any{"type": "string"},
		},
	}
}

// applySchemaFixes applies enumExtensions, optionalFields and propertyOverrides
// to the component schemas. It returns an error naming any table entry that no
// longer matches the spec, so a stale fix is noticed when the spec is refreshed.
func applySchemaFixes(schemas map[string]any) error {
	for key, values := range enumExtensions {
		schema, err := lookupSchema(schemas, key)
		if err != nil {
			return fmt.Errorf("enumExtensions[%q]: %w", key, err)
		}
		enum, ok := schema["enum"].([]any)
		if !ok {
			return fmt.Errorf("enumExtensions[%q]: schema has no enum", key)
		}
		for _, v := range values {
			if !containsValue(enum, v) {
				enum = append(enum, v)
			}
		}
		schema["enum"] = enum
	}

	for name, fields := range optionalFields {
		schema, err := lookupSchema(schemas, name)
		if err != nil {
			return fmt.Errorf("optionalFields[%q]: %w", name, err)
		}
		required, _ := schema["required"].([]any)
		for _, f := range fields {
			required = removeString(required, f)
		}
		schema["required"] = required
	}

	for key, replacement := range propertyOverrides {
		dot := strings.LastIndex(key, ".")
		parent, err := lookupSchema(schemas, key[:dot])
		if err != nil {
			return fmt.Errorf("propertyOverrides[%q]: %w", key, err)
		}
		props, ok := parent["properties"].(map[string]any)
		if !ok {
			return fmt.Errorf("propertyOverrides[%q]: parent has no properties", key)
		}
		if _, ok := props[key[dot+1:]]; !ok {
			return fmt.Errorf("propertyOverrides[%q]: property not found", key)
		}
		props[key[dot+1:]] = replacement
	}

	return nil
}

// lookupSchema resolves "Schema" or "Schema.prop.prop..." to the schema object
// it names, following "properties" at each step.
func lookupSchema(schemas map[string]any, key string) (map[string]any, error) {
	segments := strings.Split(key, ".")
	node, ok := schemas[segments[0]].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %q not found", segments[0])
	}
	for _, segment := range segments[1:] {
		props, ok := node["properties"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q has no properties", segment)
		}
		node, ok = props[segment].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("property %q not found", segment)
		}
	}
	return node, nil
}

func containsValue(values []any, v string) bool {
	for _, existing := range values {
		if existing == v {
			return true
		}
	}
	return false
}
