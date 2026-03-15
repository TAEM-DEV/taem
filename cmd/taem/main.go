// cmd/taem/main.go
//
// Cobra CLI entry point for the TAEM kernel.
// Step 12 of the kernel build order.
//
// Commands: init, launch, status, watch, retry, log, adrs

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/taem-dev/taem/internal/controller"
	"github.com/taem-dev/taem/internal/kernel"
	"github.com/taem-dev/taem/internal/state"
	"gopkg.in/yaml.v3"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd builds the top-level cobra command.
func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "taem",
		Short:   "TAEM — Mission Control Preflight Kernel",
		Long:    "TAEM kernel: the authoritative runtime for all preflight missions.\nRuns controllers, gates phases, and produces developer output.\nNo servers, no agents — one binary.",
		Version: version,
		// Silence default usage on errors so we control output.
		SilenceUsage: true,
	}

	root.AddCommand(
		initCmd(),
		launchCmd(),
		statusCmd(),
		watchCmd(),
		retryCmd(),
		logCmd(),
		adrsCmd(),
	)

	return root
}

// ---------------------------------------------------------------------------
// taem init
// ---------------------------------------------------------------------------

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize mission workspace (validate env, create .taem/)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("TAEM init — validating environment")
			fmt.Println()

			ok := true

			// Required environment variables.
			required := []struct {
				name string
				desc string
			}{
				{"TAEM_MC_STATE_PATH", "local clone of mc-state"},
				{"TAEM_ADRS_PATH", "local clone of adrs"},
				{"TAEM_ECOSYSTEM_URL", "Qdrant ecosystem URL"},
				{"TAEM_APP_ID", "GitHub App ID"},
				{"TAEM_APP_PRIVATE_KEY_PATH", "GitHub App private key PEM"},
			}

			// GH_TOKEN or GITHUB_TOKEN — at least one required.
			ghToken := os.Getenv("GH_TOKEN")
			if ghToken == "" {
				ghToken = os.Getenv("GITHUB_TOKEN")
			}
			if ghToken == "" {
				fmt.Println("  [MISSING] GH_TOKEN or GITHUB_TOKEN — GitHub API token")
				ok = false
			} else {
				fmt.Println("  [OK]      GH_TOKEN / GITHUB_TOKEN")
			}

			for _, r := range required {
				v := os.Getenv(r.name)
				if v == "" {
					fmt.Printf("  [MISSING] %s — %s\n", r.name, r.desc)
					ok = false
				} else {
					fmt.Printf("  [OK]      %s\n", r.name)
				}
			}

			// Validate mc-state path exists on disk.
			mcPath := os.Getenv("TAEM_MC_STATE_PATH")
			if mcPath != "" {
				if info, err := os.Stat(mcPath); err != nil || !info.IsDir() {
					fmt.Printf("  [WARN]    TAEM_MC_STATE_PATH (%s) does not exist or is not a directory\n", mcPath)
					ok = false
				}
			}

			// Validate adrs path exists on disk.
			adrsPath := os.Getenv("TAEM_ADRS_PATH")
			if adrsPath != "" {
				if info, err := os.Stat(adrsPath); err != nil || !info.IsDir() {
					fmt.Printf("  [WARN]    TAEM_ADRS_PATH (%s) does not exist or is not a directory\n", adrsPath)
					ok = false
				}
			}

			fmt.Println()

			// Optional services.
			fmt.Println("Optional services:")
			optional := []struct {
				name string
				desc string
			}{
				{"OLLAMA_URL", "Ollama inference endpoint"},
				{"ANTHROPIC_API_KEY", "Anthropic fallback API key"},
			}
			for _, o := range optional {
				v := os.Getenv(o.name)
				if v == "" {
					fmt.Printf("  [--]      %s — not set (%s)\n", o.name, o.desc)
				} else {
					fmt.Printf("  [OK]      %s\n", o.name)
				}
			}

			fmt.Println()

			// Create .taem/ workspace directory.
			taemDir := filepath.Join(".", ".taem")
			if err := os.MkdirAll(taemDir, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", taemDir, err)
			}
			fmt.Printf("Workspace: %s/ created\n", taemDir)

			if !ok {
				fmt.Println()
				return fmt.Errorf("environment validation failed — see [MISSING] items above")
			}

			fmt.Println()
			fmt.Println("Environment OK. Ready to launch missions.")
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// taem launch
// ---------------------------------------------------------------------------

func launchCmd() *cobra.Command {
	var (
		repos string
		task  string
		adrs  string
	)

	cmd := &cobra.Command{
		Use:   "launch",
		Short: "Launch a new preflight mission",
		RunE: func(cmd *cobra.Command, args []string) error {
			if repos == "" || task == "" || adrs == "" {
				return fmt.Errorf("--repos, --task, and --adrs are all required")
			}

			repoList := splitCSV(repos)
			adrList := splitCSV(adrs)

			fmt.Printf("Launching mission\n")
			fmt.Printf("  repos: %s\n", strings.Join(repoList, ", "))
			fmt.Printf("  task:  %s\n", task)
			fmt.Printf("  adrs:  %s\n", strings.Join(adrList, ", "))
			fmt.Println()

			// Load controller registry.
			registryPath := findRegistryPath()
			registry, err := kernel.LoadRegistry(registryPath)
			if err != nil {
				return fmt.Errorf("failed to load controller registry: %w", err)
			}
			fmt.Printf("Registry loaded: %d controllers from %s\n", len(registry.Controllers), registryPath)

			// Validate required env vars.
			mcStatePath := requireEnv("TAEM_MC_STATE_PATH")
			adrsPath := requireEnv("TAEM_ADRS_PATH")
			ecosystemURL := requireEnv("TAEM_ECOSYSTEM_URL")

			if mcStatePath == "" || adrsPath == "" || ecosystemURL == "" {
				return fmt.Errorf("required environment variables not set — run 'taem init' first")
			}

			// Set up context with signal handling (SIGINT/SIGTERM -> cancel).
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigCh
				fmt.Printf("\nReceived %s — cancelling mission\n", sig)
				cancel()
			}()

			// Launch mission via kernel.
			// kernel.go (step 11) defines Launch() and Mission.Run().
			// This will compile once kernel.go is merged.
			mission, err := kernel.LaunchFromParams(kernel.LaunchParams{
				Repos:        repoList,
				Task:         task,
				ADRs:         adrList,
				Registry:     registry,
				MCStatePath:  mcStatePath,
				ADRsPath:     adrsPath,
				EcosystemURL: ecosystemURL,
			})
			if err != nil {
				return fmt.Errorf("launch failed: %w", err)
			}

			fmt.Printf("Mission %s created\n", mission.ID)
			fmt.Println()

			// Run the mission — blocks until complete, aborted, or cancelled.
			if err := mission.Run(ctx); err != nil {
				return fmt.Errorf("mission %s failed: %w", mission.ID, err)
			}

			fmt.Printf("Mission %s complete\n", mission.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&repos, "repos", "", "Comma-separated list of repos (required)")
	cmd.Flags().StringVar(&task, "task", "", "Mission task description (required)")
	cmd.Flags().StringVar(&adrs, "adrs", "", "Comma-separated ADR IDs to check (required)")

	_ = cmd.MarkFlagRequired("repos")
	_ = cmd.MarkFlagRequired("task")
	_ = cmd.MarkFlagRequired("adrs")

	return cmd
}

// ---------------------------------------------------------------------------
// taem status
// ---------------------------------------------------------------------------

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current mission status",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcStatePath := requireEnv("TAEM_MC_STATE_PATH")
			if mcStatePath == "" {
				return fmt.Errorf("TAEM_MC_STATE_PATH not set — run 'taem init' first")
			}

			missionDir, err := findLatestMission(mcStatePath)
			if err != nil {
				return err
			}

			fmt.Printf("Mission: %s\n", filepath.Base(missionDir))
			fmt.Println()

			// Read manifest events.
			events, err := state.ReadManifest(missionDir)
			if err != nil {
				return fmt.Errorf("reading manifest: %w", err)
			}

			if len(events) == 0 {
				fmt.Println("No manifest events found.")
			} else {
				fmt.Println("Manifest events:")
				for _, evt := range events {
					phaseStr := "--"
					if evt.Phase != nil {
						phaseStr = fmt.Sprintf("%02d", *evt.Phase)
					}
					fmt.Printf("  [%s] Phase %s  %s\n", evt.Timestamp, phaseStr, evt.Event)
				}
			}

			fmt.Println()

			// Read signals.
			signals, err := state.ReadSignals(missionDir)
			if err != nil {
				return fmt.Errorf("reading signals: %w", err)
			}

			if len(signals) == 0 {
				fmt.Println("No signals recorded.")
			} else {
				fmt.Println("Controller signals:")
				for _, sig := range signals {
					fmt.Printf("  %-12s %s  %s\n", sig.Controller, padSignal(sig.SignalValue), sig.Reason)
				}
			}

			fmt.Println()

			// Determine current gate state from the latest phase.
			printGateState(events, signals)

			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// taem watch
// ---------------------------------------------------------------------------

func watchCmd() *cobra.Command {
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Live stream mission progress (polls for updates)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcStatePath := requireEnv("TAEM_MC_STATE_PATH")
			if mcStatePath == "" {
				return fmt.Errorf("TAEM_MC_STATE_PATH not set — run 'taem init' first")
			}

			missionDir, err := findLatestMission(mcStatePath)
			if err != nil {
				return err
			}

			fmt.Printf("Watching mission: %s\n", filepath.Base(missionDir))
			fmt.Printf("Polling every %s (Ctrl+C to stop)\n", interval)
			fmt.Println()

			// Set up signal handling for clean exit.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			lastSignalCount := 0
			lastEventCount := 0
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Print initial state then poll.
			for {
				signals, _ := state.ReadSignals(missionDir)
				events, _ := state.ReadManifest(missionDir)

				// Print new signals since last poll.
				if len(signals) > lastSignalCount {
					for _, sig := range signals[lastSignalCount:] {
						fmt.Printf("[SIGNAL] %-12s %s  %s\n", sig.Controller, padSignal(sig.SignalValue), sig.Reason)
					}
					lastSignalCount = len(signals)
				}

				// Print new manifest events since last poll.
				if len(events) > lastEventCount {
					for _, evt := range events[lastEventCount:] {
						phaseStr := "--"
						if evt.Phase != nil {
							phaseStr = fmt.Sprintf("%02d", *evt.Phase)
						}
						fmt.Printf("[EVENT]  Phase %s  %s\n", phaseStr, evt.Event)
					}
					lastEventCount = len(events)
				}

				// Check for terminal events.
				for _, evt := range events {
					if evt.Event == "MISSION_COMPLETE" || evt.Event == "MISSION_ABORT" {
						fmt.Printf("\nMission %s\n", evt.Event)
						return nil
					}
				}

				select {
				case <-ctx.Done():
					fmt.Println("\nWatch stopped.")
					return nil
				case <-ticker.C:
					continue
				}
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")

	return cmd
}

// ---------------------------------------------------------------------------
// taem retry
// ---------------------------------------------------------------------------

func retryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry",
		Short: "Retry a failed or escalated mission from the failed phase",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcStatePath := requireEnv("TAEM_MC_STATE_PATH")
			if mcStatePath == "" {
				return fmt.Errorf("TAEM_MC_STATE_PATH not set — run 'taem init' first")
			}

			missionDir, err := findLatestMission(mcStatePath)
			if err != nil {
				return err
			}

			missionID := filepath.Base(missionDir)

			// Read manifest to find the last phase and determine failure point.
			events, err := state.ReadManifest(missionDir)
			if err != nil {
				return fmt.Errorf("reading manifest: %w", err)
			}

			// Find the most recent PHASE_START to determine where to retry from.
			retryPhase := -1
			missionFailed := false
			for _, evt := range events {
				if evt.Event == "MISSION_ABORT" || evt.Event == "MISSION_ESCALATE" {
					missionFailed = true
				}
				if evt.Event == "PHASE_START" && evt.Phase != nil {
					retryPhase = *evt.Phase
				}
			}

			if !missionFailed {
				return fmt.Errorf("mission %s has not failed or been escalated — nothing to retry", missionID)
			}

			if retryPhase < 0 {
				return fmt.Errorf("could not determine retry phase for mission %s", missionID)
			}

			fmt.Printf("Retrying mission %s from phase %02d\n", missionID, retryPhase)
			fmt.Println()

			// Load registry.
			registryPath := findRegistryPath()
			registry, err := kernel.LoadRegistry(registryPath)
			if err != nil {
				return fmt.Errorf("failed to load controller registry: %w", err)
			}

			ecosystemURL := requireEnv("TAEM_ECOSYSTEM_URL")
			adrsPath := requireEnv("TAEM_ADRS_PATH")
			if ecosystemURL == "" || adrsPath == "" {
				return fmt.Errorf("required environment variables not set — run 'taem init' first")
			}

			// Set up context with signal handling.
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				sig := <-sigCh
				fmt.Printf("\nReceived %s — cancelling retry\n", sig)
				cancel()
			}()

			// Retry via kernel.
			// kernel.go (step 11) defines Retry() which relaunches from a given phase.
			mission, err := kernel.Retry(kernel.RetryParams{
				MissionID:    missionID,
				FromPhase:    retryPhase,
				Registry:     registry,
				MCStatePath:  mcStatePath,
				ADRsPath:     adrsPath,
				EcosystemURL: ecosystemURL,
			})
			if err != nil {
				return fmt.Errorf("retry setup failed: %w", err)
			}

			fmt.Printf("Retrying mission %s from phase %02d\n", mission.ID, retryPhase)

			if err := mission.Run(ctx); err != nil {
				return fmt.Errorf("mission %s retry failed: %w", mission.ID, err)
			}

			fmt.Printf("Mission %s retry complete\n", mission.ID)
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// taem log
// ---------------------------------------------------------------------------

func logCmd() *cobra.Command {
	var missionID string

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show mission log (signals and manifest events)",
		RunE: func(cmd *cobra.Command, args []string) error {
			mcStatePath := requireEnv("TAEM_MC_STATE_PATH")
			if mcStatePath == "" {
				return fmt.Errorf("TAEM_MC_STATE_PATH not set — run 'taem init' first")
			}

			var missionDir string
			if missionID != "" {
				missionDir = filepath.Join(mcStatePath, "missions", missionID)
				if _, err := os.Stat(missionDir); err != nil {
					return fmt.Errorf("mission %s not found at %s", missionID, missionDir)
				}
			} else {
				var err error
				missionDir, err = findLatestMission(mcStatePath)
				if err != nil {
					return err
				}
			}

			fmt.Printf("Mission log: %s\n", filepath.Base(missionDir))
			fmt.Println(strings.Repeat("=", 60))
			fmt.Println()

			// Manifest events.
			events, err := state.ReadManifest(missionDir)
			if err != nil {
				return fmt.Errorf("reading manifest: %w", err)
			}

			fmt.Println("MANIFEST")
			fmt.Println(strings.Repeat("-", 60))
			if len(events) == 0 {
				fmt.Println("  (no events)")
			}
			for _, evt := range events {
				phaseStr := "  "
				if evt.Phase != nil {
					phaseStr = fmt.Sprintf("%02d", *evt.Phase)
				}
				dur := ""
				if evt.DurationS != nil {
					dur = fmt.Sprintf(" (%ds)", *evt.DurationS)
				}
				fmt.Printf("  %s  Phase %s  %-20s%s\n", evt.Timestamp, phaseStr, evt.Event, dur)
				if evt.Task != "" {
					fmt.Printf("             task: %s\n", evt.Task)
				}
				if len(evt.Repos) > 0 {
					fmt.Printf("             repos: %s\n", strings.Join(evt.Repos, ", "))
				}
				if len(evt.ADRs) > 0 {
					fmt.Printf("             adrs: %s\n", strings.Join(evt.ADRs, ", "))
				}
			}

			fmt.Println()

			// Signals.
			signals, err := state.ReadSignals(missionDir)
			if err != nil {
				return fmt.Errorf("reading signals: %w", err)
			}

			fmt.Println("SIGNALS")
			fmt.Println(strings.Repeat("-", 60))
			if len(signals) == 0 {
				fmt.Println("  (no signals)")
			}
			for _, sig := range signals {
				fmt.Printf("  %-12s %s  %s\n", sig.Controller, padSignal(sig.SignalValue), sig.Reason)
				if len(sig.Evidence) > 0 {
					for _, e := range sig.Evidence {
						fmt.Printf("               evidence: %s\n", e)
					}
				}
				if sig.ConstraintRef != nil {
					fmt.Printf("               constraint: %s\n", *sig.ConstraintRef)
				}
				if sig.InferenceNotes != nil {
					n := sig.InferenceNotes
					fb := ""
					if n.FallbackUsed {
						fb = " (fallback)"
					}
					fmt.Printf("               inference: %s conf=%.2f %dms%s\n",
						n.Backend, n.Confidence, n.LatencyMS, fb)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&missionID, "mission", "", "Mission ID (defaults to most recent)")

	return cmd
}

// ---------------------------------------------------------------------------
// taem adrs
// ---------------------------------------------------------------------------

// adrFile represents a single ADR YAML file.
type adrFile struct {
	ID          string          `yaml:"id"`
	Title       string          `yaml:"title"`
	Status      string          `yaml:"status"`
	Enforcement string          `yaml:"enforcement"`
	Constraints []adrConstraint `yaml:"constraints"`
}

type adrConstraint struct {
	ID    string `yaml:"id"`
	Rule  string `yaml:"rule"`
	Check string `yaml:"check"`
}

func adrsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "adrs",
		Short: "List active ADRs and their constraints",
		RunE: func(cmd *cobra.Command, args []string) error {
			adrsPath := requireEnv("TAEM_ADRS_PATH")
			if adrsPath == "" {
				return fmt.Errorf("TAEM_ADRS_PATH not set — run 'taem init' first")
			}

			// Walk the ADRs directory for YAML files.
			var adrFiles []adrFile
			err := filepath.WalkDir(adrsPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				ext := filepath.Ext(path)
				if ext != ".yaml" && ext != ".yml" {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("reading %s: %w", path, err)
				}

				var adr adrFile
				if err := yaml.Unmarshal(data, &adr); err != nil {
					// Skip files that aren't valid ADR YAML.
					return nil
				}

				// Only include files that have an ID (valid ADR files).
				if adr.ID != "" {
					adrFiles = append(adrFiles, adr)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("scanning ADRs directory: %w", err)
			}

			if len(adrFiles) == 0 {
				fmt.Println("No ADR files found.")
				return nil
			}

			// Sort by ID.
			sort.Slice(adrFiles, func(i, j int) bool {
				return adrFiles[i].ID < adrFiles[j].ID
			})

			fmt.Printf("%-12s %-8s %-12s %s\n", "ID", "STATUS", "ENFORCEMENT", "TITLE")
			fmt.Println(strings.Repeat("-", 72))
			for _, adr := range adrFiles {
				status := adr.Status
				if status == "" {
					status = "unknown"
				}
				enforcement := adr.Enforcement
				if enforcement == "" {
					enforcement = "--"
				}
				constraintCount := len(adr.Constraints)
				fmt.Printf("%-12s %-8s %-12s %s (%d constraints)\n",
					adr.ID, status, enforcement, adr.Title, constraintCount)
			}

			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitCSV splits a comma-separated string into trimmed non-empty parts.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// requireEnv returns the value of an env var, printing a warning if empty.
func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "warning: %s is not set\n", key)
	}
	return v
}

// findRegistryPath returns the path to config/controllers.yaml, checking
// both the current directory and common locations.
func findRegistryPath() string {
	// Check relative to current working dir.
	if _, err := os.Stat("config/controllers.yaml"); err == nil {
		return "config/controllers.yaml"
	}

	// Check relative to the binary location (for installed binaries).
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		candidate := filepath.Join(dir, "..", "config", "controllers.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fall back to the default path.
	return "config/controllers.yaml"
}

// findLatestMission scans the mc-state missions/ directory and returns
// the path to the most recently modified mission directory.
func findLatestMission(mcStatePath string) (string, error) {
	missionsDir := filepath.Join(mcStatePath, "missions")
	entries, err := os.ReadDir(missionsDir)
	if err != nil {
		return "", fmt.Errorf("reading missions directory %s: %w", missionsDir, err)
	}

	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}

	if len(dirs) == 0 {
		return "", fmt.Errorf("no missions found in %s", missionsDir)
	}

	// Sort by name descending — mission IDs (ULIDs) sort chronologically.
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() > dirs[j].Name()
	})

	return filepath.Join(missionsDir, dirs[0].Name()), nil
}

// padSignal right-pads a signal value to a fixed width for aligned output.
func padSignal(sig string) string {
	return fmt.Sprintf("%-6s", sig)
}

// printGateState determines and prints the current gate state from manifest
// events and signals.
func printGateState(events []state.ManifestEvent, signals []controller.Signal) {
	// Find the most recent phase from manifest events.
	currentPhase := -1
	for _, evt := range events {
		if evt.Phase != nil {
			if *evt.Phase > currentPhase {
				currentPhase = *evt.Phase
			}
		}
	}

	if currentPhase < 0 {
		fmt.Println("Gate: no active phase")
		return
	}

	// Count signal types for the display.
	goCount := 0
	noGoCount := 0
	holdCount := 0
	warnCount := 0
	for _, sig := range signals {
		switch sig.SignalValue {
		case "GO":
			goCount++
		case "NO-GO":
			noGoCount++
		case "HOLD":
			holdCount++
		case "WARN":
			warnCount++
		}
	}

	fmt.Printf("Gate: Phase %02d — %d GO, %d NO-GO, %d HOLD, %d WARN\n",
		currentPhase, goCount, noGoCount, holdCount, warnCount)

	// Check for terminal states.
	for _, evt := range events {
		if evt.Event == "MISSION_COMPLETE" {
			fmt.Println("Status: LANDED")
			return
		}
		if evt.Event == "MISSION_ABORT" {
			fmt.Println("Status: ABORTED")
			return
		}
		if evt.Event == "MISSION_ESCALATE" {
			fmt.Println("Status: ESCALATED")
			return
		}
	}

	fmt.Println("Status: IN PROGRESS")
}

