package harlequin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

var names = map[string][]string{
	"dracula": {"dracula"}, "catppuccin-mocha": {"catppuccin-mocha"}, "nord": {"nord"}, "gruvbox": {"gruvbox"},
	"rose-pine":  {"rose-pine"},
	"tokyonight": {"tokyo-night"}, "tokyo-night": {"tokyo-night"}, "monokai": {"monokai"}, "solarized-light": {"solarized-light"},
}

func Adapter() model.Adapter {
	return model.Adapter{ID: "harlequin", Binary: "harlequin", Resolve: resolve, Apply: nativeApplyHarlequin}
}

func resolve(theme string, env model.Environment) (model.Port, bool, error) {
	candidates := names[shared.ThemeKey(theme)]
	if len(candidates) == 0 {
		return model.Port{}, false, nil
	}
	out, err := shared.Command(env, "harlequin", "--help")
	if err != nil {
		return model.Port{}, false, err
	}
	for _, name := range candidates {
		if nativeHarlequinHasTheme(out, name) {
			return model.Port{Kind: "native", Name: name, Source: "https://harlequin.sh/docs/themes"}, true, nil
		}
	}
	return model.Port{}, false, nil
}

func nativeHarlequinHasTheme(out []byte, name string) bool {
	help := shared.ANSI.ReplaceAllString(string(out), "")
	start := strings.Index(help, "--theme")
	if start < 0 {
		return false
	}
	section := help[start+len("--theme"):]
	if next := regexp.MustCompile(`--[a-zA-Z]`).FindStringIndex(section); next != nil {
		section = section[:next[0]]
	}
	return regexp.MustCompile(`(^|[^a-zA-Z0-9_-])` + regexp.QuoteMeta(name) + `([^a-zA-Z0-9_-]|$)`).MatchString(section)
}

func nativeHarlequinPath(env model.Environment) string {
	directory := filepath.Join(shared.NativeConfigHome(env), "harlequin")
	if env.OS == "darwin" {
		directory = filepath.Join(env.Home, "Library", "Application Support", "harlequin")
	}
	candidates := []string{filepath.Join(directory, "harlequin.toml"), filepath.Join(directory, ".harlequin.toml"), filepath.Join(directory, "config.toml"), filepath.Join(env.Home, "harlequin.toml"), filepath.Join(env.Home, ".harlequin.toml")}
	path := filepath.Join(env.Home, ".harlequin.toml")
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	return shared.NativePath("harlequin", env, path)
}

func nativeHarlequinSection(document map[string]any, path string) (map[string]any, error) {
	if filepath.Base(path) != "pyproject.toml" {
		return document, nil
	}
	tool, ok := document["tool"].(map[string]any)
	if !ok {
		if _, exists := document["tool"]; exists {
			return nil, fmt.Errorf("pyproject tool must be a table")
		}
		tool = map[string]any{}
		document["tool"] = tool
	}
	section, ok := tool["harlequin"].(map[string]any)
	if !ok {
		if _, exists := tool["harlequin"]; exists {
			return nil, fmt.Errorf("tool.harlequin must be a table")
		}
		section = map[string]any{}
		tool["harlequin"] = section
	}
	return section, nil
}

func nativeHarlequinCustom(port model.Port, env model.Environment) (string, map[string]string, error) {
	if port.File == "" {
		if port.Name == "" || strings.ContainsAny(port.Name, "\r\n\x00") {
			return "", nil, fmt.Errorf("a native Harlequin theme name is required")
		}
		return port.Name, nil, nil
	}
	path := shared.NativeExpand(port.File, env)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	document := map[string]any{}
	if err := toml.Unmarshal(data, &document); err != nil {
		return "", nil, err
	}
	section, err := nativeHarlequinSection(document, path)
	if err != nil {
		return "", nil, err
	}
	// A small native settings fragment may contain just theme = "...".
	if theme, ok := section["theme"].(string); ok && theme != "" {
		return theme, nil, nil
	}
	profiles, _ := section["profiles"].(map[string]any)
	themes := map[string]string{}
	for name, raw := range profiles {
		profile, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if theme, ok := profile["theme"].(string); ok && theme != "" {
			themes[name] = theme
		}
	}
	if len(themes) == 0 {
		return "", nil, fmt.Errorf("custom Harlequin TOML contains no profile theme settings")
	}
	return "", themes, nil
}

func nativeApplyHarlequin(port model.Port, env model.Environment) error {
	theme, custom, err := nativeHarlequinCustom(port, env)
	if err != nil {
		return err
	}
	path := nativeHarlequinPath(env)
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	document := map[string]any{}
	if err := toml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("read Harlequin config: %w", err)
	}
	section, err := nativeHarlequinSection(document, path)
	if err != nil {
		return err
	}
	profiles, ok := section["profiles"].(map[string]any)
	if !ok {
		if _, exists := section["profiles"]; exists {
			return fmt.Errorf("Harlequin profiles must be a table")
		}
		profiles = map[string]any{}
		section["profiles"] = profiles
	}
	if theme != "" {
		if len(profiles) == 0 {
			name := "themeset"
			if value, ok := section["default_profile"].(string); ok && value != "" {
				name = value
			}
			profiles[name] = map[string]any{"theme": theme}
			if _, exists := section["default_profile"]; !exists {
				section["default_profile"] = name
			}
		} else {
			for name, raw := range profiles {
				profile, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("Harlequin profile %q must be a table", name)
				}
				profile["theme"] = theme
			}
		}
	} else {
		for name, value := range custom {
			profile, ok := profiles[name].(map[string]any)
			if !ok {
				if _, exists := profiles[name]; exists {
					return fmt.Errorf("Harlequin profile %q must be a table", name)
				}
				profile = map[string]any{}
				profiles[name] = profile
			}
			profile["theme"] = value
		}
		if len(profiles) == 1 {
			if _, exists := section["default_profile"]; !exists {
				for name := range profiles {
					section["default_profile"] = name
				}
			}
		}
	}
	result, err := toml.Marshal(document)
	if err != nil {
		return err
	}
	return shared.Atomic(path, result)
}
