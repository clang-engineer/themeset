package app

import (
	"errors"
	"sort"
	"strings"

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
	ID               string   `json:"id"`
	Listed           bool     `json:"listed"`
	Tools            []string `json:"tools"`
	SupportSite      string   `json:"support_site,omitempty"`
	Appearance       string   `json:"appearance,omitempty"`
	RadarRank        int      `json:"radar_rank,omitempty"`
	RadarStars       int      `json:"radar_stars,omitempty"`
	RadarUpdatedAt   string   `json:"radar_updated_at,omitempty"`
	ProjectCreatedAt string   `json:"project_created_at,omitempty"`
	ProjectPushedAt  string   `json:"project_pushed_at,omitempty"`
}

func themeDetails(ids []string, config model.Config, ranks radarRanks) []themeDetail {
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
		standing := ranks.standing(id, config)
		detail := themeDetail{ID: id, Listed: visible[id], Tools: tools, SupportSite: catalog.SupportSite(id),
			RadarRank: standing.Rank, RadarStars: standing.Stars,
			ProjectCreatedAt: standing.CreatedAt, ProjectPushedAt: standing.PushedAt}
		if catalog.IsLight(id) {
			detail.Appearance = "light"
		}
		if detail.RadarRank != 0 {
			detail.RadarUpdatedAt = ranks.UpdatedAt
		}
		details = append(details, detail)
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
