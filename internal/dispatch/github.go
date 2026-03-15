// internal/dispatch/github.go
//
// GitHubDispatcher triggers GitHub Actions workflow_dispatch events and polls
// for run completion. Used for NAV (repo access) and PAO (repository_dispatch).
//
// Per ADR-007: GitHub Actions is a backend the kernel dispatches to — not the orchestrator.
// Per ADR-002 C-002-003: jobs must exit cleanly, no infinite polling.

package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultPollInterval is the delay between run status polls.
const defaultPollInterval = 5 * time.Second

// GitHubDispatcher dispatches to GitHub Actions and waits for completion.
type GitHubDispatcher struct {
	token        string
	appID        string
	owner        string
	repo         string
	client       *http.Client
	baseURL      string        // override for testing; empty means https://api.github.com
	pollInterval time.Duration // override for testing; zero means defaultPollInterval
}

// NewGitHubDispatcher creates a dispatcher for GitHub Actions workflows.
func NewGitHubDispatcher(token, appID, owner, repo string) *GitHubDispatcher {
	return &GitHubDispatcher{
		token:  token,
		appID:  appID,
		owner:  owner,
		repo:   repo,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// apiBase returns the base URL for API calls.
func (d *GitHubDispatcher) apiBase() string {
	if d.baseURL != "" {
		return d.baseURL
	}
	return "https://api.github.com"
}

// Dispatch triggers a workflow_dispatch event for the given workflow file.
// The inputs map is passed as the workflow inputs payload.
//
// GitHub REST API: POST /repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches
func (d *GitHubDispatcher) Dispatch(ctx context.Context, workflowFile string, inputs map[string]string) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/dispatches",
		d.apiBase(), d.owner, d.repo, workflowFile)

	payload := struct {
		Ref    string            `json:"ref"`
		Inputs map[string]string `json:"inputs"`
	}{
		Ref:    "main",
		Inputs: inputs,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal dispatch payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create dispatch request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch request failed: %w", err)
	}
	defer resp.Body.Close()

	// GitHub returns 204 No Content on success.
	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dispatch failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// workflowRun represents the relevant fields from the GitHub Actions run API.
type workflowRun struct {
	ID         int64   `json:"id"`
	Status     string  `json:"status"`     // queued, in_progress, completed
	Conclusion *string `json:"conclusion"` // success, failure, cancelled, etc.
}

// WaitForRun polls for workflow run completion with context timeout.
// Returns the conclusion string (e.g., "success", "failure") once the run completes.
//
// Per ADR-002 C-002-003: respects context cancellation, no infinite polling.
func (d *GitHubDispatcher) WaitForRun(ctx context.Context, runID int64) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d",
		d.apiBase(), d.owner, d.repo, runID)

	interval := d.pollInterval
	if interval == 0 {
		interval = defaultPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		run, err := d.fetchRun(ctx, url)
		if err != nil {
			return "", err
		}

		if run.Status == "completed" && run.Conclusion != nil {
			return *run.Conclusion, nil
		}

		// Wait for next tick or context cancellation.
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for run %d: %w", runID, ctx.Err())
		case <-ticker.C:
			// Continue polling.
		}
	}
}

// fetchRun makes a single GET request for a workflow run.
func (d *GitHubDispatcher) fetchRun(ctx context.Context, url string) (*workflowRun, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create run request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+d.token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch run failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch run failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var run workflowRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		return nil, fmt.Errorf("decode run response: %w", err)
	}

	return &run, nil
}
