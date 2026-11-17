package resource_domains

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func decodeDomain(t *testing.T) domainData {
	t.Helper()
	raw, err := os.ReadFile("../testdata/api/domains/domain-test.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Data domainData `json:"data"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	return document.Data
}

func TestApplyMapsDomainAndDNSRecords(t *testing.T) {
	data := decodeDomain(t)
	model := DomainsModel{
		AllowDowntime:      types.BoolUnknown(),
		VerificationMethod: types.StringUnknown(),
		Verify:             types.BoolValue(true),
	}
	if diags := data.apply(t.Context(), &model); diags.HasError() {
		t.Fatal(diags)
	}

	if model.CloudflareStrategy.ValueString() != "dns_proxy" {
		t.Errorf("cloudflare_strategy = %s", model.CloudflareStrategy)
	}
	if !model.WwwRedirect.IsNull() || !model.Redirect.IsNull() {
		t.Errorf("redirect = %s, www_redirect = %s, want null", model.Redirect, model.WwwRedirect)
	}
	if !model.AllowDowntime.IsNull() || !model.VerificationMethod.IsNull() {
		t.Error("unknown create-only inputs must resolve to null")
	}
	if !model.Verify.ValueBool() {
		t.Error("a configured verify must be kept")
	}

	records := model.DnsRecords
	if records.IsNull() {
		t.Fatal("dns_records should be set")
	}
	cname := records.OriginCname.Attributes()
	if !cname["value"].Equal(types.StringValue("to.laravel.cloud")) || !cname["type"].Equal(types.StringValue("CNAME")) {
		t.Errorf("origin_cname = %s", records.OriginCname)
	}
	if !records.Origin.IsNull() || !records.PreVerification.IsNull() {
		t.Error("absent records must be null")
	}
	if len(records.Ssl.Elements()) != 1 {
		t.Errorf("ssl = %s", records.Ssl)
	}
}

func TestApplyWithoutDNSRecords(t *testing.T) {
	data := decodeDomain(t)
	data.Attributes.DnsRecords = nil
	var model DomainsModel
	if diags := data.apply(t.Context(), &model); diags.HasError() {
		t.Fatal(diags)
	}
	if !model.DnsRecords.IsNull() {
		t.Error("dns_records should be null when the API omits it")
	}
}
