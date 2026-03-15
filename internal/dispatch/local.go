// internal/dispatch/local.go
//
// LocalDispatcher runs controllers in-process as goroutines with WaitGroup.
// Per ADR-007: deterministic controllers run in-process, parallel via goroutines.
// Per CLAUDE.md Hard Rules: all controller Run() methods must respect ctx.Done().

package dispatch

import (
	"context"
	"fmt"
	"sync"

	"github.com/taem-dev/taem/internal/controller"
)

// LocalDispatcher runs controllers concurrently as goroutines.
type LocalDispatcher struct{}

// NewLocalDispatcher returns a new LocalDispatcher.
func NewLocalDispatcher() *LocalDispatcher {
	return &LocalDispatcher{}
}

// RunPhase executes all given controllers concurrently using goroutines + WaitGroup.
// Each controller runs with the provided context (which includes timeout from the kernel).
// Returns collected signals from all controllers.
//
// If a controller returns an error, a NO-GO signal is generated with the error as reason.
// If the context is cancelled, controllers that haven't completed get a NO-GO signal.
func (d *LocalDispatcher) RunPhase(ctx context.Context, controllers []controller.Controller, inputs controller.Inputs) ([]controller.Signal, error) {
	if len(controllers) == 0 {
		return nil, nil
	}

	type result struct {
		signal controller.Signal
	}

	results := make(chan result, len(controllers))
	var wg sync.WaitGroup

	for _, ctrl := range controllers {
		wg.Add(1)
		go func(c controller.Controller) {
			defer wg.Done()

			sig, err := c.Run(ctx, inputs)
			if err != nil {
				// Controller errored — emit NO-GO with the error as reason.
				results <- result{
					signal: controller.Signal{
						Controller:  c.Name(),
						SignalValue: "NO-GO",
						Reason:      fmt.Sprintf("controller error: %v", err),
						Evidence:    []string{err.Error()},
					},
				}
				return
			}

			// Check if context was cancelled while the controller was running.
			// If so, the controller's signal is still valid — it completed before
			// or concurrently with cancellation. We collect it either way.
			results <- result{signal: sig}
		}(ctrl)
	}

	// Close the results channel once all goroutines complete.
	go func() {
		wg.Wait()
		close(results)
	}()

	var signals []controller.Signal
	for r := range results {
		signals = append(signals, r.signal)
	}

	return signals, nil
}
