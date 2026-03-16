package ecosystem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealth_OK verifies that Health returns nil when the ecosystem
// service responds with 200.
func TestHealth_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected path /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.Health(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

// TestHealth_Unreachable verifies that Health returns a clear error
// when the ecosystem service is unreachable per ADR-006 C-006-004.
func TestHealth_Unreachable(t *testing.T) {
	// Use a URL that will definitely fail to connect.
	c := NewClient("http://127.0.0.1:1")
	err := c.Health()
	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
	// The error should clearly indicate ecosystem is unreachable.
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}

// TestGetRepoSurface_ParsesResponse verifies that GetRepoSurface sends
// the correct request and returns the response body from the mock server.
func TestGetRepoSurface_ParsesResponse(t *testing.T) {
	expected := map[string]interface{}{
		"org":       "taem-dev",
		"repo":      "taem",
		"languages": []interface{}{"Go"},
		"stale":     false,
	}
	payload, _ := json.Marshal(expected)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/repo_surfaces/query" {
			t.Errorf("expected path /collections/repo_surfaces/query, got %s", r.URL.Path)
		}

		// Verify query parameters.
		if r.URL.Query().Get("org") != "taem-dev" {
			t.Errorf("expected org=taem-dev, got %s", r.URL.Query().Get("org"))
		}
		if r.URL.Query().Get("repo") != "taem" {
			t.Errorf("expected repo=taem, got %s", r.URL.Query().Get("repo"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(payload)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	got, err := c.GetRepoSurface("taem-dev", "taem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse both and compare.
	var gotMap map[string]interface{}
	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if gotMap["org"] != "taem-dev" || gotMap["repo"] != "taem" {
		t.Errorf("unexpected response content: %v", gotMap)
	}
}

// TestSearchLessons_PassesQuery verifies that SearchLessons sends the
// query parameter correctly to the lessons_learned collection endpoint.
func TestSearchLessons_PassesQuery(t *testing.T) {
	var receivedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/lessons/search" {
			t.Errorf("expected path /api/lessons/search, got %s", r.URL.Path)
		}
		receivedQuery = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.SearchLessons("rate limit exceeded on deploy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedQuery != "rate limit exceeded on deploy" {
		t.Errorf("expected query 'rate limit exceeded on deploy', got %q", receivedQuery)
	}
}

// TestQuery_RoutesToCorrectCollection verifies that the generic Query
// method routes to the correct collection endpoint based on the
// collection parameter.
func TestQuery_RoutesToCorrectCollection(t *testing.T) {
	collections := []string{
		"repo_surfaces",
		"mission_memory",
		"constraint_index",
		"wiring_patterns",
		"lessons_learned",
	}

	for _, collection := range collections {
		t.Run(collection, func(t *testing.T) {
			var receivedPath string
			var receivedQuery string

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				receivedQuery = r.URL.Query().Get("q")
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"results":[]}`))
			}))
			defer srv.Close()

			c := NewClient(srv.URL)
			_, err := c.Query(collection, "test query")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			endpointMap := map[string]string{
				"repo_surfaces":    "/api/repo_surfaces",
				"mission_memory":   "/api/mission_memory/search",
				"constraint_index": "/api/constraints/search",
				"wiring_patterns":  "/api/wiring_patterns/search",
				"lessons_learned":  "/api/lessons/search",
			}
			expectedPath := endpointMap[collection]
			if receivedPath != expectedPath {
				t.Errorf("expected path %s, got %s", expectedPath, receivedPath)
			}
			if receivedQuery != "test query" {
				t.Errorf("expected query 'test query', got %q", receivedQuery)
			}
		})
	}
}

// TestHealth_NonOKStatus verifies that Health returns an error when
// the ecosystem service responds with a non-200 status code.
func TestHealth_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	err := c.Health()
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

// TestSearchWiringPatterns_PassesParameters verifies that
// SearchWiringPatterns passes from, to, and via query parameters.
func TestSearchWiringPatterns_PassesParameters(t *testing.T) {
	var gotFrom, gotTo, gotVia string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/wiring_patterns/search" {
			t.Errorf("expected path /api/wiring_patterns/search, got %s", r.URL.Path)
		}
		gotFrom = r.URL.Query().Get("from")
		gotTo = r.URL.Query().Get("to")
		gotVia = r.URL.Query().Get("via")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.SearchWiringPatterns("NAV", "ARCH", "ecosystem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotFrom != "NAV" {
		t.Errorf("expected from=NAV, got %q", gotFrom)
	}
	if gotTo != "ARCH" {
		t.Errorf("expected to=ARCH, got %q", gotTo)
	}
	if gotVia != "ecosystem" {
		t.Errorf("expected via=ecosystem, got %q", gotVia)
	}
}

// TestIsRepoStale_ReturnsBool verifies that IsRepoStale correctly
// parses the stale boolean from the response.
func TestIsRepoStale_ReturnsBool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"stale":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	stale, err := c.IsRepoStale("taem-dev", "taem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stale {
		t.Error("expected stale=true, got false")
	}
}
