package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// These tests import resources from a fake Laravel Cloud API and assert that
// the plan after the import is empty, which is what adopting an existing
// resource needs. The fake API serves the recorded responses in
// testdata/api/<request path>.json; their shapes match what the live API
// returned in November 2026, with ids, names and credentials replaced.

var testProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"laravelcloud": providerserver.NewProtocol6WithError(New("test")()),
}

// fakeAPI serves recorded API responses and records the requests it receives.
type fakeAPI struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string

	// variables holds each environment's variables once they are read or
	// changed, seeded from the recorded environment. Unlike the rest of the
	// fake API they can be changed; see fake_environment_variables_test.go.
	variables map[string][]fakeVariable
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	api := &fakeAPI{}
	api.Server = httptest.NewServer(http.HandlerFunc(api.serve))
	t.Cleanup(api.Close)
	return api
}

func (a *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	a.requests = append(a.requests, r.Method+" "+r.URL.RequestURI())
	a.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer test-token" {
		http.Error(w, `{"message":"Unauthenticated."}`, http.StatusUnauthorized)
		return
	}
	if a.serveEnvironmentVariables(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"message":"the fake API is read-only"}`, http.StatusMethodNotAllowed)
		return
	}

	payload, err := readRecording(r.URL.Path)
	if err != nil {
		http.Error(w, `{"message":"Not found."}`, http.StatusNotFound)
		return
	}
	payload = a.withEnvironmentVariables(r.URL.Path, payload)

	// Like the real API, only sideload related resources when asked to.
	if r.URL.Query().Get("include") == "" {
		var document map[string]json.RawMessage
		if err := json.Unmarshal(payload, &document); err == nil {
			delete(document, "included")
			payload, _ = json.Marshal(document)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

// readRecording returns the recorded response for a request path.
func readRecording(requestPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join("testdata", "api", filepath.FromSlash(strings.TrimPrefix(requestPath, "/"))+".json"))
}

func (a *fakeAPI) requested(request string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.requests {
		if r == request {
			return true
		}
	}
	return false
}

func (a *fakeAPI) providerConfig() string {
	return `
provider "laravelcloud" {
  endpoint  = "` + a.URL + `"
  api_token = "test-token"
}
`
}

// testImportWithoutChanges imports address with importID, using config for the
// resource, and fails unless both the import command and an import block leave
// nothing to change.
func testImportWithoutChanges(t *testing.T, api *fakeAPI, address, importID, config string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:          api.providerConfig() + config,
				ResourceName:    address,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   importID,
			},
			{
				Config:             api.providerConfig() + config,
				ResourceName:       address,
				ImportState:        true,
				ImportStateId:      importID,
				ImportStatePersist: true,
			},
			{
				Config:   api.providerConfig() + config,
				PlanOnly: true,
			},
			{
				// Forget the resource instead of destroying it: the fake API is
				// read-only, and imported resources are not ours to delete.
				Config: api.providerConfig() + `
removed {
  from = ` + address + `
  lifecycle {
    destroy = false
  }
}
`,
			},
		},
	})
}

func TestImportApplication(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_applications.test", "app-test", `
resource "laravelcloud_applications" "test" {
  name       = "ember"
  region     = "eu-central-1"
  repository = "madewithlove/sales-ai"
}
`)
}

func TestImportEnvironment(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_environments.test", "app-test/env-test", `
resource "laravelcloud_environments" "test" {
  application = "app-test"
  name        = "production"
  branch      = "main"
}
`)
	if !api.requested("GET /environments/env-test?include=branch") {
		t.Error("expected the environment to be read with its branch sideloaded")
	}
}

func TestImportAppInstance(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_instances.test", "env-test/inst-test", `
resource "laravelcloud_instances" "test" {
  environment    = "env-test"
  name           = "App"
  type           = "app"
  size           = "flex-512mb"
  scaling_type   = "none"
  min_replicas   = 1
  max_replicas   = 1
  uses_scheduler = true
}
`)
}

func TestImportBackgroundProcess(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_background_processes.test", "inst-test/process-test", `
resource "laravelcloud_background_processes" "test" {
  instance  = "inst-test"
  type      = "worker"
  processes = 1
  config = {
    connection = "database"
    queue      = "crm,default"
    tries      = 1
    backoff    = 30
    sleep      = 3
    rest       = 0
    timeout    = 300
  }
}
`)
}

func TestImportLaravelMySQLCluster(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_database_clusters.test", "db-test", `
resource "laravelcloud_database_clusters" "test" {
  name   = "ember"
  type   = "laravel_mysql"
  region = "eu-central-1"
  config = {
    size                     = "mysql-flex-512mb"
    storage                  = "5"
    is_public                = "false"
    uses_scheduled_snapshots = "true"
    retention_days           = "7"
    suspend_seconds          = "300"
  }
}
`)
}

// A config listing only some settings cannot match an import, which reads every
// setting. The plan must then update the cluster in place: an earlier version
// planned to replace it, because the unset cluster_id became unknown.
func TestImportLaravelMySQLClusterWithPartialConfigUpdatesInPlace(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: api.providerConfig() + `
resource "laravelcloud_database_clusters" "test" {
  name   = "ember"
  type   = "laravel_mysql"
  region = "eu-central-1"
  config = {
    size           = "mysql-flex-512mb"
    retention_days = "7"
  }
}
`,
				ResourceName:       "laravelcloud_database_clusters.test",
				ImportState:        true,
				ImportStateKind:    resource.ImportBlockWithID,
				ImportStateId:      "db-test",
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravelcloud_database_clusters.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
}

func TestImportDomain(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_domains.test", "env-test/domain-test", `
resource "laravelcloud_domains" "test" {
  environment         = "env-test"
  name                = "ember.example.com"
  cloudflare_strategy = "dns_proxy"
}

output "origin_cname" {
  value = laravelcloud_domains.test.dns_records.origin_cname.value
}
`)
}

// key_name and key_permission are only sent on create and never read back, so
// an import leaves them null. That must not plan to replace the bucket.
func TestImportBucket(t *testing.T) {
	api := newFakeAPI(t)
	testImportWithoutChanges(t, api, "laravelcloud_buckets.test", "bucket-test", `
resource "laravelcloud_buckets" "test" {
  name           = "ember-uploads"
  visibility     = "private"
  jurisdiction   = "eu"
  key_name       = "ember"
  key_permission = "read_write"
}
`)
}

// A bucket whose keys were all deleted has no key to read back. Adopting the
// configured key inputs must then update the bucket in place, not replace it.
func TestImportBucketWithoutKeysUpdatesInPlace(t *testing.T) {
	api := newFakeAPI(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: api.providerConfig() + `
resource "laravelcloud_buckets" "test" {
  name           = "ember-uploads"
  visibility     = "private"
  jurisdiction   = "eu"
  key_name       = "ember"
  key_permission = "read_write"
}
`,
				ResourceName:       "laravelcloud_buckets.test",
				ImportState:        true,
				ImportStateKind:    resource.ImportBlockWithID,
				ImportStateId:      "bucket-without-keys",
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("laravelcloud_buckets.test", plancheck.ResourceActionUpdate),
					},
				},
			},
		},
	})
	if !api.requested("GET /buckets/bucket-without-keys?include=keys") {
		t.Error("expected the imported bucket to be read with its keys sideloaded")
	}
}
