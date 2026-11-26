package adapter

import (
	"fmt"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/model"
)

// Official URLs are source registrations, not embedded theme snapshots.
// Catppuccin Mocha consistently selects the official Mauve accent.

func filesOfficialPort(id, name string) (model.Port, bool) {
	var source string
	switch id {
	case "lazygit":
		switch name {
		case "dracula":
			source = "https://raw.githubusercontent.com/dracula/lazygit/main/config/dracula.yml"
		case "catppuccin-mocha":
			source = "https://raw.githubusercontent.com/catppuccin/lazygit/main/themes/mocha/mauve.yml"
		}
	case "lazydocker":
		if name == "dracula" {
			source = "https://raw.githubusercontent.com/dracula/lazydocker/main/config/dracula.yml"
		}
	case "atuin":
		if name == "catppuccin-mocha" {
			source = "https://raw.githubusercontent.com/catppuccin/atuin/main/themes/mocha/catppuccin-mocha-mauve.toml"
		}
	case "eza":
		if name == "catppuccin-mocha" {
			source = "https://raw.githubusercontent.com/catppuccin/eza/main/themes/mocha/catppuccin-mocha-mauve.yml"
		}
	case "yazi":
		if name == "catppuccin-mocha" {
			source = "https://raw.githubusercontent.com/catppuccin/yazi/main/themes/mocha/catppuccin-mocha-mauve.toml"
		}
	}
	if source == "" {
		return model.Port{}, false
	}
	return model.Port{Kind: "official", Name: name, Source: source}, true
}

func filesResolveLazy(id string) func(string, model.Environment) (model.Port, bool, error) {
	return func(name string, env model.Environment) (model.Port, bool, error) {
		if port, found, err := filesResolveTokyoNightPort(id, name, env); found || err != nil {
			return port, found, err
		}
		if port, found, err := filesResolveRosePinePort(id, name, env); found || err != nil {
			return port, found, err
		}
		port, ok := filesOfficialPort(id, name)
		return port, ok, nil
	}
}

func filesResolveEza(name string, env model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return model.Port{Kind: "official", Name: name, Source: filesEzaDraculaSource}, true, nil
	}
	if port, found, err := filesResolveTokyoNightPort("eza", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveRosePinePort("eza", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveEzaFamily(name, env); found || err != nil {
		return port, found, err
	}
	port, ok := filesOfficialPort("eza", name)
	return port, ok, nil
}

func filesOfficialYazi(name string) (model.Port, bool) {
	port, ok := filesOfficialPort("yazi", name)
	return port, ok
}

func filesOfficialAtuin(name string) (model.Port, bool) {
	port, ok := filesOfficialPort("atuin", name)
	return port, ok
}

func filesPortData(id string, port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" {
		return filesCustom(port, env)
	}
	if port.Kind == "official" {
		if port.Name == "tokyo-night" {
			return filesAcquireTokyoNightPort(id, port, env)
		}
		if port.Name == "rose-pine" && (id == "lazygit" || id == "atuin" || id == "eza") {
			return filesAcquireRosePinePort(id, port, env)
		}
		if id == "eza" && port.Name == "dracula" {
			return filesAcquireEzaDracula(env)
		}
		if id == "eza" && port.Name == "gruvbox" && port.Source == filesEzaGruvboxSource {
			return filesAcquireEzaFamily(port.Name, env)
		}
		if registered, ok := filesOfficialPort(id, port.Name); ok && port.Source == registered.Source {
			return filesAcquireAsset(env, port.Source)
		}
	}
	return nil, fmt.Errorf("%s requires an available official port or a registered native theme file", id)
}

// Direct theme configuration takes precedence over a flavor. Import only
// upstream Style tables, not separators, icons, filename rules or layout, and
// install the associated official syntax theme instead of the upstream example
// path. No plugin, downloader or application command is executed.
func filesApplyOfficialYazi(config, incoming map[string]any, syntaxData []byte, env model.Environment) error {
	flavor, err := filesTable(config, "flavor")
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
				target, err = filesTable(config, section)
				if err != nil {
					return err
				}
			}
			if existing, ok := target[key].(map[string]any); ok {
				filesMerge(existing, style)
			} else {
				target[key] = style
			}
		}
	}
	mgr, err := filesTable(config, "mgr")
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(filesPath("yazi", env)), "themeset-catppuccin-mocha.tmTheme")
	if err := filesAtomic(path, syntaxData); err != nil {
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
