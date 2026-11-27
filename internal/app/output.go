package app

import (
	"encoding/json"
	"fmt"
	"io"
)

func emitJSON(stdout, stderr io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	return 0
}

func printResult(stdout, stderr io.Writer, result Result) int {
	if err := printSelectionPlan(stdout, result); err != nil {
		fmt.Fprintln(stderr, "error: write output:", err)
		return 1
	}
	// Fallbacks and ordinary skips are already visible in the grouped result.
	// Keep unusual warnings and failures on stderr without repeating each row.
	for _, item := range result.Tools {
		for _, warning := range item.Warnings {
			normalSkip := item.Status == "unsupported" && item.Outcome == "skipped" &&
				(warning == "no verified native or official theme available" || warning == "no registered custom port; settings unchanged")
			normalFallback := item.FallbackTheme != "" &&
				(warning == fmt.Sprintf("theme %s unavailable for %s; using default theme %s", result.Theme, item.Tool, item.FallbackTheme) ||
					warning == fmt.Sprintf("theme %s unavailable for %s; using tool fallback theme %s", result.Theme, item.Tool, item.FallbackTheme))
			if normalSkip || normalFallback {
				continue
			}
			fmt.Fprintf(stderr, "warning: %s: %s\n", item.Tool, warning)
		}
		if item.Error != "" {
			fmt.Fprintf(stderr, "error: %s: %s\n", item.Tool, item.Error)
		}
	}
	return 0
}

// Human diagnostics are derived without changing stored or JSON warning metadata.
func printDiagnostics(stderr io.Writer, result Result) {
	unsupported := 0
	for _, item := range result.Tools {
		normalSkip := item.Status == "unsupported" && item.Outcome == "skipped" && item.Error == ""
		if normalSkip {
			unsupported++
		}
		for _, warning := range item.Warnings {
			if normalSkip && warning == "no verified native or official theme available" {
				continue
			}
			if normalSkip && warning == "no registered custom port; settings unchanged" {
				continue
			}
			fmt.Fprintf(stderr, "warning: %s: %s\n", item.Tool, warning)
		}
		if item.Error != "" {
			fmt.Fprintf(stderr, "error: %s: %s\n", item.Tool, item.Error)
		}
	}
	if unsupported > 0 {
		fmt.Fprintf(stderr, "warning: %d unsupported tools skipped (no native/official or registered custom port); settings unchanged\n", unsupported)
	}
}
