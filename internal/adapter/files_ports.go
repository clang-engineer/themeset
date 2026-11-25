package adapter

import (
	_ "embed"
	"fmt"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/model"
)

// Assets are exact upstream files. Sibling LICENSE files pin their provenance.
// Catppuccin Mocha consistently selects the official Mauve accent.

//go:embed assets/lazygit-dracula.yml
var filesLazygitDracula []byte

//go:embed assets/lazydocker-dracula.yml
var filesLazydockerDracula []byte

//go:embed assets/lazygit-catppuccin-mocha.yml
var filesLazygitCatppuccin []byte

//go:embed assets/atuin-catppuccin-mocha.toml
var filesAtuinCatppuccin []byte

//go:embed assets/eza-catppuccin-mocha.yml
var filesEzaCatppuccin []byte

//go:embed assets/yazi-catppuccin-mocha.toml
var filesYaziCatppuccin []byte

//go:embed assets/yazi-catppuccin-mocha.tmTheme
var filesYaziCatppuccinSyntax []byte

func filesOfficialPort(id, name string) (model.Port, []byte, bool) {
	var source string
	var data []byte
	switch id {
	case "lazygit":
		switch name {
		case "dracula":
			source = "https://github.com/dracula/lazygit/blob/f60ffa9e53d011739d86f0dcb681efacf3bc3cfb/config/dracula.yml"
			data = filesLazygitDracula
		case "catppuccin-mocha":
			source = "https://github.com/catppuccin/lazygit/blob/1a4b26cbc2b5be05cc5494d0d7f764120a2cf86d/themes/mocha/mauve.yml"
			data = filesLazygitCatppuccin
		}
	case "lazydocker":
		if name == "dracula" {
			source = "https://github.com/dracula/lazydocker/blob/a699a726fab2f2c6925211606f0a28151d0562db/config/dracula.yml"
			data = filesLazydockerDracula
		}
	case "atuin":
		if name == "catppuccin-mocha" {
			source = "https://github.com/catppuccin/atuin/blob/68aa64b77573c235044b614e752a781701af4eec/themes/mocha/catppuccin-mocha-mauve.toml"
			data = filesAtuinCatppuccin
		}
	case "eza":
		if name == "catppuccin-mocha" {
			source = "https://github.com/catppuccin/eza/blob/70f805f6cc27fa5b91750b75afb4296a0ec7fec9/themes/mocha/catppuccin-mocha-mauve.yml"
			data = filesEzaCatppuccin
		}
	case "yazi":
		if name == "catppuccin-mocha" {
			source = "https://github.com/catppuccin/yazi/blob/d62802be39210ea10e54b3e3b09735c6cb9e57c1/themes/mocha/catppuccin-mocha-mauve.toml"
			data = filesYaziCatppuccin
		}
	}
	if data == nil {
		return model.Port{}, nil, false
	}
	return model.Port{Kind: "official", Name: name, Source: source}, data, true
}

func filesResolveLazy(id string) func(string, model.Environment) (model.Port, bool, error) {
	return func(name string, _ model.Environment) (model.Port, bool, error) {
		port, _, ok := filesOfficialPort(id, name)
		return port, ok, nil
	}
}

func filesResolveEza(name string, _ model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return model.Port{Kind: "official", Name: name, Source: filesEzaDraculaSource}, true, nil
	}
	port, _, ok := filesOfficialPort("eza", name)
	return port, ok, nil
}

func filesOfficialYazi(name string) (model.Port, bool) {
	port, _, ok := filesOfficialPort("yazi", name)
	return port, ok
}

func filesOfficialAtuin(name string) (model.Port, bool) {
	port, _, ok := filesOfficialPort("atuin", name)
	return port, ok
}

func filesPortData(id string, port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" {
		return filesCustom(port, env)
	}
	if port.Kind == "official" {
		if id == "eza" && port.Name == "dracula" {
			return filesAcquireEzaDracula(env)
		}
		if _, data, ok := filesOfficialPort(id, port.Name); ok {
			return data, nil
		}
	}
	return nil, fmt.Errorf("%s requires an available official port or a registered native theme file", id)
}

// Direct theme configuration takes precedence over a flavor. Import only
// upstream Style tables, not separators, icons, filename rules or layout, and
// install the associated official syntax theme instead of the upstream example
// path. No plugin, downloader or application command is executed.
func filesApplyOfficialYazi(config, incoming map[string]any, env model.Environment) error {
	flavor, err := filesTable(config, "flavor")
	if err != nil {
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
	if err := filesAtomic(path, filesYaziCatppuccinSyntax); err != nil {
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
