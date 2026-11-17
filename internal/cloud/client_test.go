package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoWithIncludedDecodesDataAndIncluded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "include=branch" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{
			"data": {"id": "env-1", "attributes": {"name": "production"}},
			"included": [{"id": "42", "type": "branches", "attributes": {"name": "main"}}]
		}`))
	}))
	defer server.Close()

	var data struct {
		ID string `json:"id"`
	}
	var included []Included
	if err := New(server.URL, "token").DoWithIncluded(context.Background(), http.MethodGet, "/environments/env-1?include=branch", nil, &data, &included); err != nil {
		t.Fatal(err)
	}
	if data.ID != "env-1" {
		t.Errorf("data.ID = %q", data.ID)
	}
	if len(included) != 1 || included[0].Type != "branches" || string(included[0].Attributes) != `{"name": "main"}` {
		t.Errorf("included = %+v", included)
	}
}

func TestDoReturnsAPIErrorWithStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not found."}`, http.StatusNotFound)
	}))
	defer server.Close()

	err := New(server.URL, "token").Do(context.Background(), http.MethodGet, "/applications/missing", nil, &struct{}{})
	if !NotFound(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}
