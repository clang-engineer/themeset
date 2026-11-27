package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/clang-engineer/themeset/internal/model"
)

// plan resolves requested or fallback ports without changing tool settings.
func plan(theme string, config model.Config, env model.Environment, adapters []model.Adapter, dryRun bool) Result {
	result := Result{Theme: theme, DryRun: dryRun, Tools: make([]ToolResult, 0, len(adapters))}
	fallback := config.DefaultTheme
	if fallback == "" {
		fallback = "catppuccin-mocha"
	}
	for _, adapter := range adapters {
		item := ToolResult{Tool: adapter.ID, Outcome: "planned"}
		if !adapter.Installed() {
			item.Status, item.Outcome = "not-installed", "skipped"
			item.Reason = fmt.Sprintf("%s executable not installed", adapter.Binary)
		} else {
			item = resolveTheme(theme, config, env, adapter)
			specificFallback := config.Fallbacks[adapter.ID] != ""
			toolFallback := config.Fallbacks[adapter.ID]
			if toolFallback == "" {
				toolFallback = fallback
			}
			if item.Status == "unsupported" && toolFallback != theme {
				alternative := resolveTheme(toolFallback, config, env, adapter)
				if alternative.Status != "unsupported" {
					item = alternative
					item.FallbackTheme = toolFallback
					fallbackLabel := "default theme"
					if specificFallback {
						fallbackLabel = "tool fallback theme"
					}
					if item.Status == "error" {
						item.Error = fmt.Sprintf("%s %s: %s", fallbackLabel, toolFallback, item.Error)
					} else {
						item.Warnings = append(item.Warnings, fmt.Sprintf("theme %s unavailable for %s; using %s %s", theme, adapter.ID, fallbackLabel, toolFallback))
					}
				}
			}
		}
		result.Tools = append(result.Tools, item)
	}
	return result
}

// resolveTheme inspects exactly one theme; inspection errors never trigger fallback.
func resolveTheme(theme string, config model.Config, env model.Environment, adapter model.Adapter) ToolResult {
	item := ToolResult{Tool: adapter.ID, Outcome: "planned"}
	if custom, ok := config.Themes[theme].Custom[adapter.ID]; ok {
		custom.Kind = "custom"
		item.Status, item.Port = "custom", &custom
		if custom.URL != "" && !adapter.Remote {
			item.Error = fmt.Sprintf("%s does not accept remote theme files", adapter.ID)
			item.Status, item.Outcome, item.Port = "error", "failed", nil
			return item
		}
		if custom.File != "" {
			info, err := os.Stat(custom.File)
			if err == nil && !info.Mode().IsRegular() {
				err = errors.New("not a regular file")
			}
			if err != nil {
				item.Error = fmt.Sprintf("inspect custom file %s: %v", custom.File, err)
				item.Status, item.Outcome, item.Port = "error", "failed", nil
			}
		}
		return item
	}
	port, found, err := adapter.Resolve(DefaultPort(theme, adapter.ID), env)
	switch {
	case err != nil:
		item.Error = fmt.Sprintf("inspect native theme: %v", err)
		item.Status, item.Outcome = "error", "failed"
	case found && port.Kind != "native" && port.Kind != "official":
		item.Error = fmt.Sprintf("adapter returned invalid native port kind %q", port.Kind)
		item.Status, item.Outcome = "error", "failed"
	case found:
		item.Status, item.Port = port.Kind, &port
		if adapter.PlanWarnings != nil {
			item.Warnings = append(item.Warnings, adapter.PlanWarnings(port, env)...)
		}
	default:
		item.Status, item.Outcome = "unsupported", "skipped"
		item.Reason = unsupportedReason(adapter, theme)
		item.Warnings = append(item.Warnings, "no verified native or official theme available", "no registered custom port; settings unchanged")
	}
	return item
}

func unsupportedReason(adapter model.Adapter, theme string) string {
	if adapter.UnsupportedReason != nil {
		return adapter.UnsupportedReason(theme)
	}
	return fmt.Sprintf("no compatible installed theme or verified official port registered for %s on %s", theme, adapter.ID)
}
