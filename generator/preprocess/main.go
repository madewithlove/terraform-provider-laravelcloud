// Command preprocess rewrites the raw Laravel Cloud OpenAPI document into a
// form that is friendlier to the Terraform plugin code generators.
//
// The Cloud API speaks JSON:API, which wraps payloads in a "data" envelope at
// every level:
//
//	{ "data": { ...the actual resource... } }                 // response document
//	{ "relationships": { "server": { "data": {...} } } }      // relationships
//
// If those envelopes are fed to tfplugingen-openapi / tfplugingen-framework,
// the generated resources expose unidiomatic nested "data" attributes, and the
// framework generator emits duplicate "DataValue"/"DataType" Go types whenever
// "data" appears more than once in a single resource (it does not namespace
// nested object types by their parent), which fails to compile.
//
// This tool performs two rewrites:
//
//  1. It recursively unwraps every JSON:API "data" envelope: an object schema
//     shaped like {properties: {data: <schema>, ...reserved}} is replaced by
//     <schema>. An envelope is only unwrapped when its sibling properties are
//     all JSON:API reserved keywords (links, meta, jsonapi, included, errors),
//     so a legitimate domain field named "data" is left untouched.
//
//  2. It collapses JSON:API resource-identifier objects -- shaped like
//     {type: <const>, id: <string>} -- down to a plain string holding the id.
//     A relationship like "server" then becomes a string attribute instead of
//     a nested {id, type} object, which is both idiomatic for Terraform and
//     avoids duplicate nested-type generation when the same identifier name
//     (e.g. "role") appears under more than one parent.
//
//  3. It flattens JSON:API resource objects -- shaped like
//     {id, type, attributes: {...}, relationships: {...}, links: {...}} --
//     by hoisting the "attributes" members up to the top level and dropping the
//     "type", "links", and "relationships" members. The resulting resource
//     schema is flat, which is the idiomatic Terraform shape and makes state
//     mapping straightforward.
//
// It also genericizes discriminated unions to string maps, strips the JSON:API
// "include" query parameter, renames reserved-word fields, and injects parent
// path parameters; see the tables below.
//
// Usage:
//
//	go run ./generator/preprocess \
//	    -in generator/docs.openapi.json \
//	    -out generator/docs.openapi.processed.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// reserved holds the JSON:API top-level / relationship member names that may
// legitimately sit alongside "data" inside an envelope object.
var reserved = map[string]bool{
	"links":    true,
	"meta":     true,
	"jsonapi":  true,
	"included": true,
	"errors":   true,
}

// responseFixes corrects response schema references that are wrong in the
// upstream OpenAPI document. The key is "METHOD PATH" and the value maps a
// status code to the component schema the response should actually reference.
// Both endpoints below are async (202) operations whose response erroneously
// references the wrong schema.
var responseFixes = map[string]map[string]string{
	// Add entries when an endpoint's response references the wrong schema, e.g.:
	//   "post /environments/{environment}/commands": {"202": "CommandResource"},
}

// renames maps a component schema name to property renames applied to it.
// Terraform reserves a handful of meta-argument names ("provider", "count",
// "for_each", "depends_on", "lifecycle", ...) as top-level resource attribute
// names. When a request body promotes such a property to the top level of a
// resource it must be renamed, since the OpenAPI generators have no notion of
// reserved words. Extend this table if new collisions appear.
var renames = map[string]map[string]string{
	// "connection" is a reserved root block name in Terraform; expose
	// connection details under a non-reserved name wherever they appear.
	"DatabaseResource": {"connection": "connection_details"},
	"CacheResource":    {"connection": "connection_details"},
}

// parentParams injects the parent path-parameter(s) of a nested resource as
// string attributes on its response schema. The Cloud API addresses resources
// by their own id on flat read paths (e.g. GET /environments/{environment}),
// so the parent (which only appears in the create path, e.g.
// POST /applications/{application}/environments) is never derived as an
// attribute. Injecting it here makes the generated schema and model carry the
// parent, which the resource then marks Required so it can build create paths.
// The keys are the create-request schema names (the generators build the
// resource model primarily from the create request body, so the parent must be
// injected there to appear in the model).
var parentParams = map[string][]string{
	"CreateEnvironmentRequest":         {"application"},
	"CreateInstanceRequest":            {"environment"},
	"CreateBackgroundProcessRequest":   {"instance"},
	"CreateDomainRequest":              {"environment"},
	"CreateCommandRequest":             {"environment"},
	"StoreFilesystemKeyRequest":        {"filesystem"},
	"StoreDatabaseSchemaRequest":       {"database"},
	"StoreDatabaseSnapshotRequest":     {"database"},
	"StoreWebsocketApplicationRequest": {"websocketServer"},
}

// applyParentParams injects the configured parent path parameters as string
// properties on each create-request schema so they are generated as model
// fields. Resources mark them Required + RequiresReplace.
func applyParentParams(schemas map[string]any) {
	for name, parents := range parentParams {
		schema, ok := schemas[name].(map[string]any)
		if !ok {
			continue
		}
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			props = map[string]any{}
			schema["properties"] = props
		}
		for _, p := range parents {
			if _, exists := props[p]; !exists {
				props[p] = map[string]any{
					"type":        "string",
					"description": "The " + p + " this resource belongs to.",
				}
			}
		}
	}
}

func main() {
	in := flag.String("in", "generator/docs.openapi.json", "path to the raw OpenAPI document")
	out := flag.String("out", "generator/docs.openapi.processed.json", "path to write the processed OpenAPI document")
	flag.Parse()

	raw, err := os.ReadFile(*in)
	if err != nil {
		log.Fatalf("reading %s: %v", *in, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		log.Fatalf("parsing %s: %v", *in, err)
	}

	applyResponseFixes(doc)
	stripParams(doc, "include")

	c := &counts{}
	// Only rewrite the parts the generators actually consume: the reusable
	// component schemas and the request/response schemas referenced by paths.
	if components, ok := doc["components"].(map[string]any); ok {
		schemas, _ := components["schemas"].(map[string]any)
		applyRenames(schemas)
		applyParentParams(schemas)
		components["schemas"] = rewrite(schemas, c)
	}
	doc["paths"] = rewrite(doc["paths"], c)

	processed, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		log.Fatalf("encoding processed document: %v", err)
	}

	if err := os.WriteFile(*out, processed, 0o644); err != nil {
		log.Fatalf("writing %s: %v", *out, err)
	}

	fmt.Printf("unwrapped %d envelope(s), collapsed %d identifier(s), genericized %d union(s), flattened %d resource(s); wrote %s\n",
		c.envelopes, c.identifiers, c.unions, c.resources, *out)
}

// stripParams removes query parameters with the given name from every
// operation. The JSON:API "include" sideload parameter, for instance, is not a
// resource field and should not become a Terraform attribute.
func stripParams(doc map[string]any, name string) {
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return
	}
	for _, item := range paths {
		operations, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for _, op := range operations {
			operation, ok := op.(map[string]any)
			if !ok {
				continue
			}
			params, ok := operation["parameters"].([]any)
			if !ok {
				continue
			}
			kept := params[:0]
			for _, p := range params {
				param, ok := p.(map[string]any)
				if ok && param["name"] == name && param["in"] == "query" {
					continue
				}
				kept = append(kept, p)
			}
			operation["parameters"] = kept
		}
	}
}

// applyResponseFixes points wrong response references at the correct component
// schema, in place, before any other rewrite runs. It only touches the inner
// "data" reference of a JSON:API response envelope.
func applyResponseFixes(doc map[string]any) {
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		return
	}
	for key, byCode := range responseFixes {
		fields := strings.SplitN(key, " ", 2)
		method, path := fields[0], fields[1]

		item, ok := paths[path].(map[string]any)
		if !ok {
			continue
		}
		op, ok := item[method].(map[string]any)
		if !ok {
			continue
		}
		responses, ok := op["responses"].(map[string]any)
		if !ok {
			continue
		}
		for code, schemaName := range byCode {
			resp, ok := responses[code].(map[string]any)
			if !ok {
				continue
			}
			content, ok := resp["content"].(map[string]any)
			if !ok {
				continue
			}
			for _, media := range content {
				m, ok := media.(map[string]any)
				if !ok {
					continue
				}
				schema, ok := m["schema"].(map[string]any)
				if !ok {
					continue
				}
				if data, ok := schema["properties"].(map[string]any); ok {
					if d, ok := data["data"].(map[string]any); ok {
						d["$ref"] = "#/components/schemas/" + schemaName
					}
				}
			}
		}
	}
}

// applyRenames renames request/response properties that would otherwise become
// reserved Terraform attribute names. It rewrites both the property key and any
// matching entry in the schema's "required" list.
func applyRenames(schemas map[string]any) {
	for schemaName, props := range renames {
		schema, ok := schemas[schemaName].(map[string]any)
		if !ok {
			continue
		}
		// Apply renames against the schema's own properties and, for JSON:API
		// resource schemas, against the nested "attributes" object (which is
		// later flattened to the top level).
		targets := []map[string]any{schema}
		if properties, ok := schema["properties"].(map[string]any); ok {
			if attrs, ok := properties["attributes"].(map[string]any); ok {
				targets = append(targets, attrs)
			}
		}
		for from, to := range props {
			for _, target := range targets {
				renameProperty(target, from, to)
			}
		}
	}
}

// renameProperty renames a property within a schema object, updating its
// "required" list to match.
func renameProperty(schema map[string]any, from, to string) {
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return
	}
	value, ok := properties[from]
	if !ok {
		return
	}
	properties[to] = value
	delete(properties, from)

	if required, ok := schema["required"].([]any); ok {
		for i, name := range required {
			if name == from {
				required[i] = to
			}
		}
	}
}

// counts tracks how many of each rewrite were applied.
type counts struct {
	envelopes   int
	identifiers int
	unions      int
	resources   int
}

// rewrite walks an arbitrary JSON node, transforming children first, then
// applying the schema rewrites: collapsing JSON:API "data" envelopes and
// resource-identifier objects, and flattening JSON:API resource objects.
func rewrite(node any, c *counts) any {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			n[k] = rewrite(v, c)
		}
		if inner, ok := envelopeInner(n); ok {
			c.envelopes++
			return inner
		}
		if id, ok := identifierString(n); ok {
			c.identifiers++
			return id
		}
		if g, ok := genericizeUnion(n); ok {
			c.unions++
			return g
		}
		if flattenResource(n) {
			c.resources++
		}
		return n
	case []any:
		for i, v := range n {
			n[i] = rewrite(v, c)
		}
		return n
	default:
		return node
	}
}

// genericizeUnion collapses a discriminated polymorphic schema (a oneOf/anyOf
// with a "discriminator") into a permissive string map. The generators do not
// support schema composition, so a field like a database's type-specific
// "config" would otherwise cause the whole resource to be skipped. A
// map(string) is a workable, generic representation. Only discriminated unions
// are targeted, so nullability unions (anyOf: [type, null]) are left alone.
func genericizeUnion(schema map[string]any) (map[string]any, bool) {
	if _, ok := schema["discriminator"]; !ok {
		return nil, false
	}
	_, oneOf := schema["oneOf"]
	_, anyOf := schema["anyOf"]
	if !oneOf && !anyOf {
		return nil, false
	}
	out := map[string]any{
		"type":                 "object",
		"additionalProperties": map[string]any{"type": "string"},
	}
	if desc, ok := schema["description"]; ok {
		out["description"] = desc
	}
	return out, true
}

// flattenResource reports whether schema is a JSON:API resource object and, if
// so, flattens it in place: the "attributes" members are hoisted to the top
// level and the "type", "links", and "relationships" members are dropped.
//
// Unlike a relationship-flattening approach, "relationships" are dropped rather
// than hoisted: in this API relationship members frequently share a name with a
// request/response attribute (e.g. an instance's "background_processes"
// relationship vs. the request field), which would collide into duplicate
// attributes. Parent references are already exposed as path-parameter
// attributes, so dropping the relationship links loses nothing actionable.
func flattenResource(schema map[string]any) bool {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return false
	}
	// A resource object always carries a "type" discriminator alongside an
	// "attributes" object; that pair distinguishes it from request bodies and
	// other plain objects.
	if _, ok := props["type"]; !ok {
		return false
	}
	if _, ok := props["attributes"].(map[string]any); !ok {
		return false
	}

	required, _ := schema["required"].([]any)
	required = removeString(required, "type")

	if attrs, ok := props["attributes"].(map[string]any); ok {
		if nestedProps, ok := attrs["properties"].(map[string]any); ok {
			for name, def := range nestedProps {
				props[name] = def
			}
		}
		if nestedReq, ok := attrs["required"].([]any); ok {
			required = append(required, nestedReq...)
		}
	}

	delete(props, "attributes")
	delete(props, "relationships")
	delete(props, "type")
	delete(props, "links")

	if len(required) > 0 {
		schema["required"] = required
	} else {
		delete(schema, "required")
	}
	return true
}

// removeString returns the slice with every occurrence of s removed.
func removeString(in []any, s string) []any {
	out := in[:0]
	for _, v := range in {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// identifierString reports whether schema is a JSON:API resource-identifier
// object -- an object whose properties are limited to "id", "type" and "meta"
// and which carries an "id" -- and if so returns a plain string schema holding
// the identifier, preserving any description.
func identifierString(schema map[string]any) (map[string]any, bool) {
	if t, _ := schema["type"].(string); t != "object" {
		return nil, false
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, false
	}
	if _, ok := props["id"]; !ok {
		return nil, false
	}
	for name := range props {
		switch name {
		case "id", "type", "meta":
		default:
			return nil, false
		}
	}
	out := map[string]any{"type": "string"}
	if desc, ok := schema["description"]; ok {
		out["description"] = desc
	}
	return out, true
}

// envelopeInner reports whether schema is a JSON:API "data" envelope and, if
// so, returns the schema nested under "data". An envelope is an object whose
// "properties" contain "data" and whose remaining properties are all reserved
// JSON:API member names.
func envelopeInner(schema map[string]any) (any, bool) {
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, false
	}
	data, ok := props["data"]
	if !ok {
		return nil, false
	}
	for name := range props {
		if name != "data" && !reserved[name] {
			return nil, false
		}
	}
	return data, true
}
