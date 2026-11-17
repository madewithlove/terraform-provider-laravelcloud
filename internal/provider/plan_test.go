package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// These tests check a plan for a new resource. The fake API is read-only, so
// the create that follows fails, and they need no recorded responses.

// The buckets resource sends cors_settings to the API but never reads it back,
// so every field left out of the configuration has to plan as null. An unknown
// value would still be unknown after apply, which Terraform rejects.
func TestBucketPartialCorsSettingsPlanAsNull(t *testing.T) {
	api := newFakeAPI(t)
	cors := tfjsonpath.New("cors_settings")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: api.providerConfig() + `
resource "laravelcloud_buckets" "test" {
  name           = "ember"
  visibility     = "private"
  jurisdiction   = "default"
  key_name       = "ember"
  key_permission = "read_write"
  cors_settings = {
    allowed_origins = ["https://ember.example.com"]
  }
}
`,
				// The fake API is read-only, so the apply after the plan checks fails.
				ExpectError: regexp.MustCompile("Unable to create bucket"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("laravelcloud_buckets.test", cors.AtMapKey("allowed_origins"),
							knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("https://ember.example.com")})),
						plancheck.ExpectKnownValue("laravelcloud_buckets.test", cors.AtMapKey("allowed_headers"), knownvalue.Null()),
						plancheck.ExpectKnownValue("laravelcloud_buckets.test", cors.AtMapKey("allowed_methods"), knownvalue.Null()),
						plancheck.ExpectKnownValue("laravelcloud_buckets.test", cors.AtMapKey("expose_headers"), knownvalue.Null()),
						plancheck.ExpectKnownValue("laravelcloud_buckets.test", cors.AtMapKey("max_age_seconds"), knownvalue.Null()),
					},
				},
			},
		},
	})
}
