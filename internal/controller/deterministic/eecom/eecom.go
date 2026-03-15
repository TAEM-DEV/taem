// Package eecom implements the EECOM (Resource Monitor) controller.
//
// EECOM checks the GitHub API rate_limit endpoint. WARN at < 25%
// headroom, NO-GO at < 10%. Arithmetic against thresholds, no inference.
package eecom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/taem-dev/taem/internal/controller"
)

// Thresholds for rate limit headroom.
const (
	warnThreshold = 0.25 // WARN below 25% remaining
	noGoThreshold = 0.10 // NO-GO below 10% remaining
)

// rateLimitResponse models the GitHub rate_limit API response.
type rateLimitResponse struct {
	Resources struct {
		Core struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		} `json:"core"`
	} `json:"resources"`
}

// HTTPClient is an interface for HTTP calls, allowing test injection.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Controller implements the EECOM (Resource Monitor) deterministic controller.
type Controller struct {
	Client  HTTPClient
	BaseURL string // overridable for testing, defaults to https://api.github.com
}

// New returns a new EECOM controller instance with the default HTTP client.
func New() *Controller {
	return &Controller{
		Client:  http.DefaultClient,
		BaseURL: "https://api.github.com",
	}
}

// Name returns the controller callsign.
func (c *Controller) Name() string {
	return "EECOM"
}

// Mode returns ModeDeterministic per ADR-005.
func (c *Controller) Mode() controller.ControllerMode {
	return controller.ModeDeterministic
}

// Run checks the GitHub API rate limit and returns GO, WARN, or NO-GO.
func (c *Controller) Run(ctx context.Context, inputs controller.Inputs) (controller.Signal, error) {
	select {
	case <-ctx.Done():
		return controller.Signal{}, ctx.Err()
	default:
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}

	url := c.BaseURL + "/rate_limit"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return controller.Signal{}, fmt.Errorf("eecom: create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("rate_limit endpoint unreachable: %v", err),
			Evidence:    []string{fmt.Sprintf("error=%v", err)},
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("failed to read rate_limit response: %v", err),
			Evidence:    []string{fmt.Sprintf("read_error=%v", err)},
		}, nil
	}

	var rl rateLimitResponse
	if err := json.Unmarshal(body, &rl); err != nil {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("invalid rate_limit response: %v", err),
			Evidence:    []string{fmt.Sprintf("parse_error=%v", err)},
		}, nil
	}

	limit := rl.Resources.Core.Limit
	remaining := rl.Resources.Core.Remaining

	if limit == 0 {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "NO-GO",
			Reason:      "rate limit reports zero capacity",
			Evidence:    []string{"limit=0"},
		}, nil
	}

	headroom := float64(remaining) / float64(limit)
	evidence := []string{
		fmt.Sprintf("limit=%d", limit),
		fmt.Sprintf("remaining=%d", remaining),
		fmt.Sprintf("headroom=%.2f%%", headroom*100),
	}

	if headroom < noGoThreshold {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "NO-GO",
			Reason:      fmt.Sprintf("rate limit headroom %.1f%% below 10%% threshold", headroom*100),
			Evidence:    evidence,
		}, nil
	}

	if headroom < warnThreshold {
		return controller.Signal{
			Controller:  "EECOM",
			SignalValue: "WARN",
			Reason:      fmt.Sprintf("rate limit headroom %.1f%% below 25%% threshold", headroom*100),
			Evidence:    evidence,
		}, nil
	}

	return controller.Signal{
		Controller:  "EECOM",
		SignalValue: "GO",
		Reason:      fmt.Sprintf("rate limit headroom %.1f%% — nominal", headroom*100),
		Evidence:    evidence,
	}, nil
}
