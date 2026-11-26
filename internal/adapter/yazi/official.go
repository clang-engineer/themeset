package yazi

import (
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// Direct theme configuration takes precedence over a flavor. Import only
// upstream Style tables, not separators, icons, filename rules or layout, and
// install the associated official syntax theme instead of the upstream example
// path. No plugin, downloader or application command is executed.
func filesApplyOfficialYazi(config, incoming map[string]any, syntaxData []byte, env model.Environment) error {
	flavor, err := shared.Table(config, "flavor")
	if err != nil {
		return err
	}
	if err := filesRemovePreviousYaziStyles(config, env); err != nil {
		return err
	}
	flavor["dark"] = ""
	flavor["light"] = ""
	delete(flavor, "use")
	for _, section := range []string{"app", "mgr", "tabs", "mode", "indicator", "status", "input", "pick", "confirm", "spot", "completion", "tasks", "which", "help", "notify"} {
		styles, ok := incoming[section].(map[string]any)
		if !ok {
			continue
		}
		var target map[string]any
		for key, value := range styles {
			style, ok := value.(map[string]any)
			if !ok || !filesYaziStyle(style) {
				continue
			}
			if target == nil {
				target, err = shared.Table(config, section)
				if err != nil {
					return err
				}
			}
			if existing, ok := target[key].(map[string]any); ok {
				shared.Merge(existing, style)
			} else {
				target[key] = style
			}
		}
	}
	mgr, err := shared.Table(config, "mgr")
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(shared.Path("yazi", env)), "themeset-catppuccin-mocha.tmTheme")
	if err := shared.Atomic(path, syntaxData); err != nil {
		return err
	}
	mgr["syntect_theme"] = path
	return nil
}

func filesYaziStyle(style map[string]any) bool {
	if len(style) == 0 {
		return false
	}
	for key := range style {
		switch key {
		case "fg", "bg", "bold", "dim", "italic", "underline", "blink", "blink_rapid", "reversed", "hidden", "crossed":
		default:
			return false
		}
	}
	return true
}
