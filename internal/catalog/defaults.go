package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

//go:embed defaults.json
var bundledThemeData []byte

type listedTheme struct {
	ID          string `json:"id"`
	SupportSite string `json:"support_site,omitempty"`
}

type bundledThemes struct {
	Listed []listedTheme                         `json:"listed"`
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

// Port returns the bundled defaults for a theme and tool. Do not mutate the
// returned declaration: it is shared for the lifetime of the process.
func Port(theme, tool string) model.ThemePort {
	return defaults.Themes[theme][tool]
}
