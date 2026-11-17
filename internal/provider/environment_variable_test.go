package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These tests manage environment variables on the fake API, which keeps the
// variables of env-test in memory. The recorded environment has one variable,
// APP_ENV, that Terraform does not manage and must leave alone.

func environmentVariableConfig(api *fakeAPI, name, key, value string) string {
	return api.providerConfig() + fmt.Sprintf(`
resource "laravelcloud_environment_variable" %q {
  environment = "env-test"
  key         = %q
  value       = %q
}
`, name, key, value)
}

// checkVariable checks a variable's value in the fake API.
func checkVariable(api *fakeAPI, key, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		value, ok := api.variable("env-test", key)
		if !ok {
			return fmt.Errorf("%s is not set on env-test", key)
		}
		if value != want {
			return fmt.Errorf("%s = %q, want %q", key, value, want)
		}
		return nil
	}
}

// checkNoVariable checks that a variable is not set in the fake API.
func checkNoVariable(api *fakeAPI, key string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if _, ok := api.variable("env-test", key); ok {
			return fmt.Errorf("%s is still set on env-test", key)
		}
		return nil
	}
}

func TestEnvironmentVariableLifecycle(t *testing.T) {
	api := newFakeAPI(t)
	address := "laravelcloud_environment_variable.token"
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				// Create.
				Config: environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "first"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "environment", "env-test"),
					resource.TestCheckResourceAttr(address, "key", "CRM_WEBSITE_TOKEN"),
					resource.TestCheckResourceAttr(address, "value", "first"),
					checkVariable(api, "CRM_WEBSITE_TOKEN", "first"),
					checkVariable(api, "APP_ENV", "production"),
				),
			},
			{
				// A new value updates the variable in place.
				Config: environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(address, "value", "second"),
					checkVariable(api, "CRM_WEBSITE_TOKEN", "second"),
					checkVariable(api, "APP_ENV", "production"),
				),
			},
			{
				// A value changed outside Terraform shows as drift and is set back.
				PreConfig: func() { api.setVariable("env-test", "CRM_WEBSITE_TOKEN", "changed in the dashboard") },
				Config:    environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate),
					},
				},
				Check: checkVariable(api, "CRM_WEBSITE_TOKEN", "second"),
			},
			{
				// A variable deleted outside Terraform is created again.
				PreConfig: func() { api.deleteVariable("env-test", "CRM_WEBSITE_TOKEN") },
				Config:    environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreate),
					},
				},
				Check: checkVariable(api, "CRM_WEBSITE_TOKEN", "second"),
			},
			{
				// A new key replaces the variable: the old key is deleted.
				Config: environmentVariableConfig(api, "token", "WEBSITE_TOKEN", "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					checkNoVariable(api, "CRM_WEBSITE_TOKEN"),
					checkVariable(api, "WEBSITE_TOKEN", "second"),
					checkVariable(api, "APP_ENV", "production"),
				),
			},
		},
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			checkNoVariable(api, "WEBSITE_TOKEN"),
			checkVariable(api, "APP_ENV", "production"),
		),
	})
}

// Deleting one variable leaves the environment's other variables alone, both
// the ones Terraform manages and the ones it does not.
func TestEnvironmentVariableDeleteLeavesOtherKeysAlone(t *testing.T) {
	api := newFakeAPI(t)
	both := environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "secret") + `
resource "laravelcloud_environment_variable" "url" {
  environment = "env-test"
  key         = "CRM_WEBSITE_URL"
  value       = "https://example.com"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: both,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkVariable(api, "CRM_WEBSITE_TOKEN", "secret"),
					checkVariable(api, "CRM_WEBSITE_URL", "https://example.com"),
				),
			},
			{
				Config: environmentVariableConfig(api, "token", "CRM_WEBSITE_TOKEN", "secret"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravelcloud_environment_variable.url", plancheck.ResourceActionDestroy),
						plancheck.ExpectResourceAction("laravelcloud_environment_variable.token", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					checkNoVariable(api, "CRM_WEBSITE_URL"),
					checkVariable(api, "CRM_WEBSITE_TOKEN", "secret"),
					checkVariable(api, "APP_ENV", "production"),
				),
			},
		},
		CheckDestroy: checkVariable(api, "APP_ENV", "production"),
	})
}

// Creating a variable that is already set must not overwrite it silently.
func TestEnvironmentVariableCreateRefusesAnExistingKey(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      environmentVariableConfig(api, "app_env", "APP_ENV", "staging"),
				ExpectError: regexp.MustCompile(`(?s)APP_ENV is already set on environment env-test.*"env-test/APP_ENV"`),
			},
		},
	})
	if value, _ := api.variable("env-test", "APP_ENV"); value != "production" {
		t.Errorf("APP_ENV = %q, want it left at production", value)
	}
}

func TestEnvironmentVariableRejectsAnInvalidKey(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      environmentVariableConfig(api, "invalid", "CRM-WEBSITE-TOKEN", "secret"),
				ExpectError: regexp.MustCompile(`must start with a letter or underscore`),
			},
		},
	})
}

func TestImportEnvironmentVariable(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_environment_variable.test", "env-test/APP_ENV",
		environmentVariableConfig(api, "test", "APP_ENV", "production")[len(api.providerConfig()):])
}

func TestImportMissingEnvironmentVariableFails(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        environmentVariableConfig(api, "test", "CRM_WEBSITE_TOKEN", "secret"),
				ResourceName:  "laravelcloud_environment_variable.test",
				ImportState:   true,
				ImportStateId: "env-test/CRM_WEBSITE_TOKEN",
				ExpectError:   regexp.MustCompile(`Cannot import non-existent remote object`),
			},
		},
	})
}

func TestImportEnvironmentVariableRejectsAMalformedID(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:        environmentVariableConfig(api, "test", "APP_ENV", "production"),
				ResourceName:  "laravelcloud_environment_variable.test",
				ImportState:   true,
				ImportStateId: "APP_ENV",
				ExpectError:   regexp.MustCompile(`environment-id/KEY`),
			},
		},
	})
}
