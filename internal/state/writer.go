package state

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/taem-dev/taem/internal/controller"
)

// maxPushRetries is the number of retry-with-rebase attempts before
// giving up on git push. Per ADR-004 C-004-007.
const maxPushRetries = 3

// retryDelay is the base delay between push retries. Each retry doubles.
const retryDelay = 2 * time.Second

// AppendSignal appends a single signal as one JSON line to signals.jsonl
// in the given mission directory. Creates the file if it does not exist.
func AppendSignal(missionDir string, signal controller.Signal) error {
	return appendJSONL(filepath.Join(missionDir, signalsFile), signal)
}

// AppendManifest appends a single manifest event as one JSON line to
// manifest.jsonl in the given mission directory. Creates the file if it
// does not exist.
func AppendManifest(missionDir string, event ManifestEvent) error {
	return appendJSONL(filepath.Join(missionDir, manifestFile), event)
}

// WriteIntegrationMap writes the raw bytes to integration-map.json,
// overwriting any existing content. The file is created with 0644 perms.
func WriteIntegrationMap(missionDir string, data []byte) error {
	return os.WriteFile(filepath.Join(missionDir, integrationMapFile), data, 0644)
}

// WriteStepPlan writes the raw bytes to step-plan.json, overwriting any
// existing content. The file is created with 0644 perms.
func WriteStepPlan(missionDir string, data []byte) error {
	return os.WriteFile(filepath.Join(missionDir, stepPlanFile), data, 0644)
}

// GitCommitAndPush stages all changes in the mission directory, commits
// with the given message, and pushes to origin. If the push fails due to
// a non-fast-forward (concurrent update), it retries with rebase up to
// maxPushRetries times per ADR-004 C-004-007.
//
// IMPORTANT: git push --force is NEVER used on mc-state.
func GitCommitAndPush(missionDir string, message string) error {
	// Resolve the git repo root — missionDir may be a subdirectory.
	repoRoot, err := gitRepoRoot(missionDir)
	if err != nil {
		return fmt.Errorf("state: git repo root: %w", err)
	}

	// Stage changes.
	if err := gitCmd(repoRoot, "add", "."); err != nil {
		return fmt.Errorf("state: git add: %w", err)
	}

	// Commit (allow empty is not used — if nothing to commit, that's an error
	// the caller should avoid).
	if err := gitCmd(repoRoot, "commit", "-m", message); err != nil {
		return fmt.Errorf("state: git commit: %w", err)
	}

	// Push with retry-with-rebase.
	delay := retryDelay
	for attempt := 0; attempt <= maxPushRetries; attempt++ {
		err := gitCmd(repoRoot, "push", "origin", "HEAD")
		if err == nil {
			return nil
		}

		if attempt == maxPushRetries {
			return fmt.Errorf("state: git push failed after %d retries: %w", maxPushRetries, err)
		}

		// Pull --rebase to incorporate remote changes before retrying.
		if rebaseErr := gitCmd(repoRoot, "pull", "--rebase", "origin", "HEAD"); rebaseErr != nil {
			return fmt.Errorf("state: git pull --rebase failed: %w (original push error: %v)", rebaseErr, err)
		}

		time.Sleep(delay)
		delay *= 2
	}

	// Unreachable, but satisfy the compiler.
	return fmt.Errorf("state: git push: exhausted retries")
}

// appendJSONL marshals v as compact JSON and appends it as a single line
// to the given file path. Creates the file if it does not exist.
func appendJSONL(path string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("state: json marshal: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("state: open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("state: write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// gitCmd runs a git command in the given directory and returns any error.
func gitCmd(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr // git output goes to stderr for logging
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitRepoRoot finds the git repository root for a given path.
func gitRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim trailing newline.
	root := string(out)
	if len(root) > 0 && root[len(root)-1] == '\n' {
		root = root[:len(root)-1]
	}
	return root, nil
}
