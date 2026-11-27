package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/clang-engineer/themeset/internal/model"
)

//go:embed defaults.json
var bundledThemeData []byte

type listedTheme struct {
	ID          string `json:"id"`
	SupportSite string `json:"support_site,omitempty"`
	RadarRepo   string `json:"radar_repo,omitempty"`
}

type bundledThemes struct {
	Listed []listedTheme                         `json:"listed"`
	Light  []string                              `json:"light"`
	Themes map[string]map[string]model.ThemePort `json:"themes"`
}

var defaults = func() bundledThemes {
	var data bundledThemes
	if err := json.Unmarshal(bundledThemeData, &data); err != nil {
		panic(fmt.Sprintf("invalid bundled themes: %v", err))
	}
	for name, tools := range data.Themes {
		for _, spec := range tools {
			if spec.Official == nil {
				continue
			}
			spec.Official.Kind = "official"
			if spec.Official.Name == "" {
				spec.Official.Name = name
			}
		}
	}
	return data
}()

func Listed() []string {
	ids := make([]string, len(defaults.Listed))
	for i, theme := range defaults.Listed {
		ids[i] = theme.ID
	}
	return ids
}

// Registered reports whether a bundled theme has any declared tool ports,
// including themes intentionally hidden from the default picker.
func Registered(theme string) bool {
	_, ok := defaults.Themes[theme]
	return ok
}

// ThemeIDs returns every bundled theme ID, including entries omitted from the
// default picker. The caller owns the returned sorted slice.
func ThemeIDs() []string {
	ids := make([]string, 0, len(defaults.Themes))
	for id := range defaults.Themes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func SupportSite(theme string) string {
	for _, entry := range defaults.Listed {
		if entry.ID == theme {
			return entry.SupportSite
		}
	}
	return ""
}

// RadarRepo identifies the family project used by Radar for a listed theme.
// It is ranking metadata, not a source for application theme assets.
func RadarRepo(theme string) string {
	for _, entry := range defaults.Listed {
		if entry.ID == theme {
			return entry.RadarRepo
		}
	}
	return ""
}

// IsLight reports explicitly labeled bundled light themes, including hidden ones.
// An unlisted appearance is unknown, not necessarily dark.
func IsLight(theme string) bool {
	for _, id := range defaults.Light {
		if id == theme {
			return true
		}
	}
	return false
}

// Tools lists declared tool ports, not installed or currently applicable tools.
func Tools(theme string) []string {
	ports := defaults.Themes[theme]
	ids := make([]string, 0, len(ports))
	for id := range ports {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

var verifiedPorts = func() map[string]map[model.Port]struct{} {
	byTool := make(map[string]map[model.Port]struct{})
	for _, tools := range defaults.Themes {
		for tool, spec := range tools {
			if spec.Official == nil {
				continue
			}
			if byTool[tool] == nil {
				byTool[tool] = make(map[model.Port]struct{})
			}
			byTool[tool][*spec.Official] = struct{}{}
		}
	}
	return byTool
}()

// Verified reports whether a port is exactly one of the bundled official
// declarations. User config cannot add an entry to this registry.
func Verified(tool string, port model.Port) bool {
	_, ok := verifiedPorts[tool][port]
	return ok
}

// KnownNative reports whether a native theme name appears in bundled data.
func KnownNative(tool, name string) bool {
	for _, ports := range defaults.Themes {
		for id, spec := range ports {
			if id != tool {
				continue
			}
			for _, native := range spec.Native {
				if native == name {
					return true
				}
			}
		}
	}
	return false
}

// Port returns the bundled defaults for a theme and tool. Do not mutate the
// returned declaration: it is shared for the lifetime of the process.
func Port(theme, tool string) model.ThemePort {
	return defaults.Themes[theme][tool]
}
