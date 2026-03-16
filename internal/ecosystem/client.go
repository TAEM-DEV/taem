// internal/ecosystem/client.go
//
// Read-only Qdrant HTTP client for the 5 ecosystem collections per ADR-006.
// The kernel reads from ecosystem via this client — it never writes directly
// to Qdrant (C-006-002). Writes happen via the ecosystem service API.
//
// Collections: repo_surfaces, mission_memory, constraint_index,
//              wiring_patterns, lessons_learned.
//
// If ecosystem is unreachable, methods return clear errors so NAV can
// emit HOLD per C-006-004.
//
// Uses only stdlib (net/http, encoding/json). No external Qdrant client.

package ecosystem

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// EcosystemClient is a read-only HTTP client for the TAEM ecosystem
// service that fronts 5 Qdrant collections.
type EcosystemClient struct {
	baseURL    string
	httpClient *http.Client
}

// staleResponse is the JSON shape returned by the staleness endpoint.
type staleResponse struct {
	Stale bool `json:"stale"`
}

// NewClient creates a new EcosystemClient pointing at the given base URL.
func NewClient(baseURL string) *EcosystemClient {
	return &EcosystemClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Health checks whether the ecosystem service is reachable.
// Returns nil on success, or a clear error per ADR-006 C-006-004.
func (c *EcosystemClient) Health() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("ecosystem unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ecosystem unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// GetRepoSurface reads a cached repo surface from the repo_surfaces
// collection. NAV uses this to avoid cold-reading repos.
func (c *EcosystemClient) GetRepoSurface(org, repo string) ([]byte, error) {
	u := c.baseURL + "/collections/repo_surfaces/query"
	params := url.Values{}
	params.Set("org", org)
	params.Set("repo", repo)

	return c.doGet(u, params)
}

// IsRepoStale checks whether the cached surface for a repo is stale.
func (c *EcosystemClient) IsRepoStale(org, repo string) (bool, error) {
	u := c.baseURL + "/collections/repo_surfaces/stale"
	params := url.Values{}
	params.Set("org", org)
	params.Set("repo", repo)

	body, err := c.doGet(u, params)
	if err != nil {
		return false, err
	}

	var sr staleResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return false, fmt.Errorf("ecosystem: failed to parse stale response: %w", err)
	}
	return sr.Stale, nil
}

// SearchMissionMemory searches prior missions in the mission_memory
// collection. PRB-Skeptic uses this for historical context.
func (c *EcosystemClient) SearchMissionMemory(query string) ([]byte, error) {
	return c.searchCollection("mission_memory", query)
}

// SearchConstraints searches the constraint_index collection.
// ARCH and PRB-ADR use this to validate against active constraints.
func (c *EcosystemClient) SearchConstraints(query string) ([]byte, error) {
	return c.searchCollection("constraint_index", query)
}

// SearchWiringPatterns searches validated integration patterns in the
// wiring_patterns collection. NAV and ARCH use this for pattern matching.
func (c *EcosystemClient) SearchWiringPatterns(from, to, via string) ([]byte, error) {
	u := c.baseURL + "/api/wiring_patterns/search"
	params := url.Values{}
	params.Set("from", from)
	params.Set("to", to)
	params.Set("via", via)

	return c.doGet(u, params)
}

// SearchLessons searches the lessons_learned collection.
// PRB-Skeptic uses this to check for prior failures.
func (c *EcosystemClient) SearchLessons(query string) ([]byte, error) {
	return c.searchCollection("lessons_learned", query)
}

// Query is the generic query function wired into Inputs.EcosystemQuery.
// Controllers call it with a collection name and free-text query.
func (c *EcosystemClient) Query(collection, query string) ([]byte, error) {
	if collection == "health" {
		if err := c.Health(); err != nil {
			return nil, err
		}
		return []byte(`{"status":"ok"}`), nil
	}
	return c.searchCollection(collection, query)
}

// searchCollection is the shared implementation for collection searches.
// Routes to the correct ecosystem service API endpoints.
func (c *EcosystemClient) searchCollection(collection, query string) ([]byte, error) {
	endpointMap := map[string]string{
		"repo_surfaces":    "/api/repo_surfaces",
		"mission_memory":   "/api/mission_memory/search",
		"constraint_index": "/api/constraints/search",
		"wiring_patterns":  "/api/wiring_patterns/search",
		"lessons_learned":  "/api/lessons/search",
	}
	endpoint, ok := endpointMap[collection]
	if !ok {
		endpoint = "/api/" + collection + "/search"
	}
	u := c.baseURL + endpoint
	params := url.Values{}
	params.Set("q", query)

	return c.doGet(u, params)
}

// doGet performs an HTTP GET with query parameters and returns the response body.
// Returns a clear error if the ecosystem service is unreachable (C-006-004).
func (c *EcosystemClient) doGet(rawURL string, params url.Values) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("ecosystem: invalid URL %q: %w", rawURL, err)
	}
	u.RawQuery = params.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("ecosystem unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ecosystem: collection query failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ecosystem: failed to read response body: %w", err)
	}
	return body, nil
}
