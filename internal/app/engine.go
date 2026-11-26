package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

var builtinThemes = catalog.Listed()

// ToolResult separates support selection from the actual application outcome.
type ToolResult struct {
	Tool          string      `json:"tool"`
	Status        string      `json:"status"`
	Outcome       string      `json:"outcome"`
	Port          *model.Port `json:"port,omitempty"`
	FallbackTheme string      `json:"fallback_theme,omitempty"`
	Warnings      []string    `json:"warnings,omitempty"`
	Reason        string      `json:"reason,omitempty"`
	Error         string      `json:"error,omitempty"`
}

type Result struct {
	Theme     string       `json:"theme"`
	DryRun    bool         `json:"dry_run"`
	AppliedAt string       `json:"applied_at,omitempty"`
	Tools     []ToolResult `json:"tools"`
}

func themeIDs(config model.Config) []string {
	if config.Listed != nil {
		return config.Listed
	}
	set := make(map[string]bool, len(builtinThemes)+len(config.Themes))
	for _, name := range builtinThemes {
		set[name] = true
	}
	for name := range config.Themes {
		set[name] = true
	}
	ids := make([]string, 0, len(set))
	for name := range set {
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids
}

// allThemeIDs exposes the full bundled registry as well as names supplied by
// config, even when a listed override hides all default discovery entries.
func allThemeIDs(config model.Config) []string {
	ids := catalog.ThemeIDs()
	seen := make(map[string]bool, len(ids)+len(config.Themes)+len(config.Listed))
	for _, id := range ids {
		seen[id] = true
	}
	for id := range config.Themes {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, id := range config.Listed {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Strings(ids)
	return ids
}

type themeDetail struct {
	ID          string   `json:"id"`
	Listed      bool     `json:"listed"`
	Tools       []string `json:"tools"`
	SupportSite string   `json:"support_site,omitempty"`
}

func themeDetails(ids []string, config model.Config) []themeDetail {
	visible := make(map[string]bool)
	for _, id := range themeIDs(config) {
		visible[id] = true
	}
	details := make([]themeDetail, 0, len(ids))
	for _, id := range ids {
		tools := catalog.Tools(id)
		if custom := config.Themes[id].Custom; len(custom) != 0 {
			set := make(map[string]bool, len(tools)+len(custom))
			for _, tool := range tools {
				set[tool] = true
			}
			for tool := range custom {
				if !set[tool] {
					tools = append(tools, tool)
				}
			}
			sort.Strings(tools)
		}
		if tools == nil {
			tools = []string{}
		}
		details = append(details, themeDetail{id, visible[id], tools, catalog.SupportSite(id)})
	}
	return details
}

// Discovery can hide a theme without revoking explicitly requested applications.
func knownTheme(theme string, config model.Config) bool {
	if _, ok := config.Themes[theme]; ok {
		return true
	}
	if catalog.Registered(theme) {
		return true
	}
	for _, name := range builtinThemes {
		if name == theme {
			return true
		}
	}
	for _, name := range config.Listed {
		if name == theme {
			return true
		}
	}
	return false
}

func selectTools(ids []string, adapters []model.Adapter) ([]model.Adapter, error) {
	byID := make(map[string]model.Adapter, len(adapters))
	for _, adapter := range adapters {
		byID[adapter.ID] = adapter
	}
	if ids == nil {
		ids = make([]string, 0, len(adapters))
		for _, adapter := range adapters {
			ids = append(ids, adapter.ID)
		}
	}
	selected := make([]model.Adapter, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		adapter, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q (use 'list tools' to list IDs; Neovim's ID is neovim)", id)
		}
		if !seen[id] {
			selected = append(selected, adapter)
			seen[id] = true
		}
	}
	// Stable order does not depend on map iteration or user duplicate ordering.
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, nil
}

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
			if item.Status == "unsupported" && fallback != theme {
				alternative := resolveTheme(fallback, config, env, adapter)
				if alternative.Status != "unsupported" {
					item = alternative
					item.FallbackTheme = fallback
					if item.Status == "error" {
						item.Error = fmt.Sprintf("default theme %s: %s", fallback, item.Error)
					} else {
						item.Warnings = append(item.Warnings, fmt.Sprintf("theme %s unavailable for %s; using default theme %s", theme, adapter.ID, fallback))
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
	default:
		item.Status, item.Outcome = "unsupported", "skipped"
		item.Reason = unsupportedReason(adapter.ID, theme)
		item.Warnings = append(item.Warnings, "no verified native or official theme available", "no registered custom port; settings unchanged")
	}
	return item
}

func unsupportedReason(tool, theme string) string {
	switch tool {
	case "tmux":
		return "no registered styles-only theme; full plugins can replace the existing status layout"
	case "yazi":
		return "no complete compatible installed flavor or pinned official flavor recipe"
	case "lazydocker":
		return "no compatible native theme or verified lazydocker port registered"
	}
	return fmt.Sprintf("no compatible installed theme or verified official port registered for %s on %s", theme, tool)
}

func apply(result *Result, env model.Environment, adapters []model.Adapter) {
	byID := make(map[string]model.Adapter, len(adapters))
	for _, adapter := range adapters {
		byID[adapter.ID] = adapter
	}
	for i := range result.Tools {
		item := &result.Tools[i]
		if item.Port == nil {
			continue
		}
		if err := byID[item.Tool].Apply(*item.Port, env); err != nil {
			item.Status, item.Outcome = "error", "failed"
			message := fmt.Sprintf("apply theme: %v", err)
			if item.Error != "" {
				message = item.Error + "; " + message
			}
			item.Error = message
		} else {
			item.Outcome = "applied"
		}
	}
	result.AppliedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func failed(result Result) bool {
	for _, item := range result.Tools {
		if item.Error != "" {
			return true
		}
	}
	return false
}

func statePath(env model.Environment) string {
	return filepath.Join(env.ConfigHome, "themeset", "state.json")
}

func saveState(path string, result Result) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace symlink %s", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".state-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func readState(path string) (*Result, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result Result
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func splitTools(value string) ([]string, error) {
	ids := []string{}
	if value == "" {
		return ids, nil
	}
	for _, part := range strings.Split(value, ",") {
		id := strings.TrimSpace(part)
		if id == "" {
			return nil, errors.New("--tools contains an empty tool ID")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
