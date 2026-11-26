package eza

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "eza", Binary: "eza", Resolve: filesResolveEza, Apply: filesApplyEza}
}

func filesApplyEza(port model.Port, env model.Environment) error {
	data, err := filesPortData(port, env)
	if err != nil {
		return err
	}
	if port.Kind == "official" && port.File == "" && port.Name == "catppuccin-mocha" {
		if err := filesValidateEzaPort(port.Name, data); err != nil {
			return err
		}
	}
	incoming, err := shared.YAML(data)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		return errors.New("eza theme file is empty")
	}
	return shared.EditYAML(shared.Path("eza", env), func(config map[string]any) error {
		shared.Merge(config, incoming)
		return nil
	})
}

// The upstream Rosé Pine distribution has no \"colourful\" toggle; validate its
// native eza style tables without imposing Gruvbox's additional setting.
func rosePineValidateEza(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return err
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("Rosé Pine eza requires native %s styles", section)
		}
		for key, style := range styles {
			if err := filesValidateEzaFamilyStyle(section+"."+key, style); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay"} {
		if err := filesValidateEzaFamilyStyle(key, config[key]); err != nil {
			return err
		}
	}
	return nil
}

func filesValidateEzaPort(theme string, data []byte) error {
	switch theme {
	case "catppuccin-mocha", "rose-pine":
		return rosePineValidateEza(data)
	case "tokyo-night":
		return filesValidateEzaFamily(data)
	default:
		return fmt.Errorf("unsupported eza port %q", theme)
	}
}

// Acquisition is apply-only. Validate the upstream theme before touching settings.
func filesAcquireEzaDracula(env model.Environment) ([]byte, error) {
	data, err := shared.AcquireAsset(env, shared.EzaDraculaSource)
	if err != nil {
		return nil, fmt.Errorf("acquire eza Dracula: %w", err)
	}
	if err := filesValidateEzaDracula(data); err != nil {
		return nil, err
	}
	if _, err := shared.AcquireAsset(env, shared.EzaDraculaLicenseSource); err != nil {
		return nil, fmt.Errorf("acquire eza Dracula license: %w", err)
	}
	return data, nil
}

// Check the native upstream structure before the caller merges it into the
// active YAML. In particular, arbitrary nonempty YAML (such as lazygit's gui
// palette) is not an eza theme.
func filesValidateEzaDracula(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return fmt.Errorf("parse eza Dracula YAML: %w", err)
	}
	if colourful, ok := config["colourful"].(bool); !ok || !colourful {
		return fmt.Errorf("eza Dracula requires native colourful: true")
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("eza Dracula requires native %s styles", section)
		}
		for name, value := range styles {
			if err := filesValidateEzaDraculaStyle(section+"."+name, value); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay"} {
		if err := filesValidateEzaDraculaStyle(key, config[key]); err != nil {
			return err
		}
	}
	return nil
}

func filesValidateEzaDraculaStyle(name string, value any) error {
	style, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("eza Dracula %s must be a native style mapping", name)
	}
	foreground, ok := style["foreground"].(string)
	if !ok || foreground == "" {
		return fmt.Errorf("eza Dracula %s requires a foreground color", name)
	}
	return nil
}

func filesResolveEzaFamily(name string, _ model.Environment) (model.Port, bool, error) {
	if name != "gruvbox" {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: filesEzaGruvboxSource}, true, nil
}

// Called only on apply, after the resolver has registered an exact source.
// The caller merges the validated eza styles into the effective theme.yml.
func filesAcquireEzaFamily(name string, env model.Environment) ([]byte, error) {
	if name != "gruvbox" {
		return nil, fmt.Errorf("no registered eza family asset for %q", name)
	}
	data, err := shared.AcquireAsset(env, filesEzaGruvboxSource)
	if err != nil {
		return nil, fmt.Errorf("acquire eza Gruvbox Dark: %w", err)
	}
	if err := filesValidateEzaFamily(data); err != nil {
		return nil, err
	}
	if _, err := shared.AcquireAsset(env, filesEzaFamilyLicenseSource); err != nil {
		return nil, fmt.Errorf("acquire eza Gruvbox Dark license: %w", err)
	}
	return data, nil
}

var filesEzaFamilyHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate the eza theme.yml style tables, rather than merely accepting YAML
// that could belong to another application.
func filesValidateEzaFamily(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return fmt.Errorf("parse eza Gruvbox Dark YAML: %w", err)
	}
	if colourful, ok := config["colourful"].(bool); !ok || !colourful {
		return fmt.Errorf("eza Gruvbox Dark requires colourful: true")
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("eza Gruvbox Dark requires native %s styles", section)
		}
		for key, style := range styles {
			if err := filesValidateEzaFamilyStyle(section+"."+key, style); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay"} {
		if err := filesValidateEzaFamilyStyle(key, config[key]); err != nil {
			return err
		}
	}
	return nil
}

func filesValidateEzaFamilyStyle(name string, value any) error {
	style, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("eza Gruvbox Dark %s must be a native style mapping", name)
	}
	foreground, ok := style["foreground"].(string)
	if !ok || !filesEzaFamilyHexColor.MatchString(foreground) {
		return fmt.Errorf("eza Gruvbox Dark %s requires a hexadecimal foreground color", name)
	}
	for key, value := range style {
		switch key {
		case "foreground", "background":
			color, ok := value.(string)
			if !ok || !filesEzaFamilyHexColor.MatchString(color) {
				return fmt.Errorf("eza Gruvbox Dark %s.%s requires a hexadecimal color", name, key)
			}
		case "is_bold", "is_dimmed", "is_italic", "is_underline", "is_blink", "is_reverse", "is_hidden", "is_strikethrough", "prefix_with_reset":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("eza Gruvbox Dark %s.%s requires a boolean", name, key)
			}
		default:
			return fmt.Errorf("eza Gruvbox Dark %s.%s is not a native style property", name, key)
		}
	}
	return nil
}

func filesResolveEza(name string, env model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return model.Port{Kind: "official", Name: name, Source: shared.EzaDraculaSource}, true, nil
	}
	if port, found, err := shared.ResolveTokyoNightPort("eza", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := shared.ResolveRosePinePort("eza", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveEzaFamily(name, env); found || err != nil {
		return port, found, err
	}
	port, ok := shared.OfficialPort("eza", name)
	return port, ok, nil
}

func filesPortData(port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" {
		return shared.Custom(port, env)
	}
	if port.Kind == "official" {
		switch port.Name {
		case "tokyo-night":
			data, err := shared.AcquireTokyoNightPort("eza", port, env)
			if err != nil {
				return nil, err
			}
			if err := filesValidateEzaPort("tokyo-night", data); err != nil {
				return nil, fmt.Errorf("invalid Tokyo Night eza port: %w", err)
			}
			return data, nil
		case "rose-pine":
			data, err := shared.AcquireRosePinePort("eza", port, env)
			if err != nil {
				return nil, err
			}
			if err := filesValidateEzaPort("rose-pine", data); err != nil {
				return nil, fmt.Errorf("invalid Rosé Pine eza theme: %w", err)
			}
			return data, nil
		case "dracula":
			if port.Source == shared.EzaDraculaSource {
				return filesAcquireEzaDracula(env)
			}
		case "gruvbox":
			if port.Source == filesEzaGruvboxSource {
				return filesAcquireEzaFamily(port.Name, env)
			}
		default:
			if registered, ok := shared.OfficialPort("eza", port.Name); ok && port.Source == registered.Source {
				return shared.AcquireAsset(env, port.Source)
			}
		}
	}
	return nil, fmt.Errorf("eza requires an available official port or a registered native theme file")
}
