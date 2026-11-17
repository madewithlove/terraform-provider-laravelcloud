package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccImportExisting imports existing Laravel Cloud resources and asserts
// the plan afterwards is empty. It only plans an import (with configuration
// generated from the imported state), so it never creates, changes or
// destroys anything, and is safe to point at production resources.
//
// It runs with TF_ACC=1 and LARAVEL_CLOUD_TOKEN set; each resource is tested
// when its import ID is set:
//
//	LARAVEL_CLOUD_ACC_APPLICATION         app-...
//	LARAVEL_CLOUD_ACC_ENVIRONMENT         app-.../env-...
//	LARAVEL_CLOUD_ACC_INSTANCE            env-.../inst-...
//	LARAVEL_CLOUD_ACC_BACKGROUND_PROCESS  inst-.../process-...
//	LARAVEL_CLOUD_ACC_DATABASE_CLUSTER    db-...
//	LARAVEL_CLOUD_ACC_DOMAIN              env-.../domain-...
func TestAccImportExisting(t *testing.T) {
	cases := map[string]string{
		"LARAVEL_CLOUD_ACC_APPLICATION":        "laravelcloud_applications.imported",
		"LARAVEL_CLOUD_ACC_ENVIRONMENT":        "laravelcloud_environments.imported",
		"LARAVEL_CLOUD_ACC_INSTANCE":           "laravelcloud_instances.imported",
		"LARAVEL_CLOUD_ACC_BACKGROUND_PROCESS": "laravelcloud_background_processes.imported",
		"LARAVEL_CLOUD_ACC_DATABASE_CLUSTER":   "laravelcloud_database_clusters.imported",
		"LARAVEL_CLOUD_ACC_DOMAIN":             "laravelcloud_domains.imported",
	}
	for variable, address := range cases {
		t.Run(address, func(t *testing.T) {
			importID := os.Getenv(variable)
			if importID == "" {
				t.Skipf("%s is not set", variable)
			}
			resource.Test(t, resource.TestCase{
				PreCheck: func() {
					if os.Getenv("LARAVEL_CLOUD_TOKEN") == "" {
						t.Fatal("LARAVEL_CLOUD_TOKEN must be set for acceptance tests")
					}
				},
				ProtoV6ProviderFactories: testProviderFactories,
				Steps: []resource.TestStep{
					{
						Config:          `provider "laravelcloud" {}`,
						ResourceName:    address,
						ImportState:     true,
						ImportStateKind: resource.ImportBlockWithID,
						ImportStateId:   importID,
						GenerateConfig:  true,
					},
				},
			})
		})
	}
}
