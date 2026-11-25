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

	"github.com/clang-engineer/themeset/internal/model"
)

var builtinThemes = []string{
	"catppuccin-mocha", "cyberdream", "dracula", "everforest",
	"github-dark-high-contrast", "gruvbox", "gruvbox-hard", "kanagawa",
	"modus-vivendi", "moonfly", "nord", "onedark", "rose-pine", "sonokai",
	"tokyonight", "vscode-dark",
}

// ToolResult separates support selection from the actual application outcome.
// Error may coexist with an applied custom fallback after a native inspection error.
type ToolResult struct {
	Tool     string      `json:"tool"`
	Status   string      `json:"status"`
	Outcome  string      `json:"outcome"`
	Port     *model.Port `json:"port,omitempty"`
	Warnings []string    `json:"warnings,omitempty"`
	Error    string      `json:"error,omitempty"`
}

type Result struct {
	Theme     string       `json:"theme"`
	DryRun    bool         `json:"dry_run"`
	AppliedAt string       `json:"applied_at,omitempty"`
	Tools     []ToolResult `json:"tools"`
}

func themeIDs(config model.Config) []string {
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
	for _, adapter := range adapters {
		item := ToolResult{Tool: adapter.ID, Outcome: "planned"}
		if !adapter.Installed() {
			item.Status, item.Outcome = "not-installed", "skipped"
			result.Tools = append(result.Tools, item)
			continue
		}
		port, found, err := adapter.Resolve(theme, env)
		if err != nil {
			item.Error = fmt.Sprintf("inspect native theme: %v", err)
			found = false
		}
		if found && port.Kind != "native" && port.Kind != "official" {
			item.Error = fmt.Sprintf("adapter returned invalid native port kind %q", port.Kind)
			found = false
		}
		if found {
			item.Status, item.Port = port.Kind, &port
		} else {
			item.Warnings = append(item.Warnings, "no verified native or official theme available")
			if custom, ok := config.Themes[theme].Custom[adapter.ID]; ok {
				custom.Kind = "custom"
				item.Status, item.Port = "custom", &custom
				if custom.File != "" {
					info, fileErr := os.Stat(custom.File)
					if fileErr == nil && !info.Mode().IsRegular() {
						fileErr = errors.New("not a regular file")
					}
					if fileErr != nil {
						message := fmt.Sprintf("inspect custom file %s: %v", custom.File, fileErr)
						if item.Error != "" {
							message = item.Error + "; " + message
						}
						item.Error = message
						item.Status, item.Outcome, item.Port = "error", "failed", nil
					}
				}
			} else if item.Error != "" {
				item.Status, item.Outcome = "error", "failed"
			} else {
				item.Status, item.Outcome = "unsupported", "skipped"
				item.Warnings = append(item.Warnings, "no registered custom port; settings unchanged")
			}
		}
		result.Tools = append(result.Tools, item)
	}
	return result
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
