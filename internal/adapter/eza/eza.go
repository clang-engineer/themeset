package eza

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
	"gopkg.in/yaml.v3"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "eza", Binary: "eza", Remote: true, Resolve: filesResolveEza, Apply: filesApplyEza}
}

func filesApplyEza(port model.Port, env model.Environment) error {
	data, err := filesPortData(port, env)
	if err != nil {
		return err
	}
	if port.URL != "" {
		if err := filesValidateEzaCustom(data); err != nil {
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
	return shared.EditYAML(shared.Path("eza", env), func(config *yaml.Node) error {
		return shared.MergeYAMLMap(config, incoming)
	})
}

// Remote custom data must describe eza styles, not an unrelated YAML config.
// Local custom files retain their existing permissive merge behavior.
func filesValidateEzaCustom(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return err
	}
	styles := 0
	for key, value := range config {
		switch key {
		case "colourful":
			if _, ok := value.(bool); !ok {
				return errors.New("eza colourful must be a boolean")
			}
		case "filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type":
			section, ok := value.(map[string]any)
			if !ok || len(section) == 0 {
				return fmt.Errorf("eza %s must contain native styles", key)
			}
			for name, style := range section {
				if err := filesValidateEzaCustomStyle(key+"."+name, style); err != nil {
					return err
				}
				styles++
			}
		case "punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay":
			if err := filesValidateEzaCustomStyle(key, value); err != nil {
				return err
			}
			styles++
		case "filenames", "extensions":
			section, ok := value.(map[string]any)
			if !ok || len(section) == 0 {
				return fmt.Errorf("eza %s must contain native styles", key)
			}
			for name, value := range section {
				entry, ok := value.(map[string]any)
				if !ok || len(entry) == 0 {
					return fmt.Errorf("eza %s.%s must be a native style mapping", key, name)
				}
				for field, style := range entry {
					switch field {
					case "filename":
						if err := filesValidateEzaCustomStyle(key+"."+name+".filename", style); err != nil {
							return err
						}
					case "icon":
						icon, ok := style.(map[string]any)
						if !ok || len(icon) == 0 {
							return fmt.Errorf("eza %s.%s.icon must be a mapping", key, name)
						}
						for property, value := range icon {
							switch property {
							case "glyph":
								if glyph, ok := value.(string); !ok || glyph == "" || strings.ContainsFunc(glyph, unicode.IsControl) {
									return fmt.Errorf("eza %s.%s.icon.glyph must be nonempty without control characters", key, name)
								}
							case "style":
								if err := filesValidateEzaCustomStyle(key+"."+name+".icon.style", value); err != nil {
									return err
								}
							default:
								return fmt.Errorf("eza %s.%s.icon.%s is not a native property", key, name, property)
							}
						}
					default:
						return fmt.Errorf("eza %s.%s.%s is not a native style", key, name, field)
					}
					styles++
				}
			}
		default:
			return fmt.Errorf("eza %s is not a native theme setting", key)
		}
	}
	if styles == 0 {
		return errors.New("eza theme contains no native styles")
	}
	return nil
}

func filesValidateEzaCustomStyle(name string, value any) error {
	style, ok := value.(map[string]any)
	if !ok || len(style) == 0 {
		return fmt.Errorf("eza %s must be a nonempty native style mapping", name)
	}
	for key, value := range style {
		switch key {
		case "foreground", "background":
			if color, ok := value.(string); !ok || !filesEzaFamilyHexColor.MatchString(color) {
				return fmt.Errorf("eza %s.%s must be a hexadecimal color", name, key)
			}
		case "is_bold", "is_dimmed", "is_italic", "is_underline", "is_blink", "is_reverse", "is_hidden", "is_strikethrough", "prefix_with_reset":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("eza %s.%s must be a boolean", name, key)
			}
		default:
			return fmt.Errorf("eza %s.%s is not a native style property", name, key)
		}
	}
	return nil
}

// The flat eza style format has no "colourful" toggle; validate its native
// style tables without imposing the family format's additional setting.
func filesValidateEzaFlat(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return err
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("flat eza theme requires native %s styles", section)
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

func filesValidateEzaPort(port model.Port, data []byte) error {
	switch port.Format {
	case "eza-flat":
		return filesValidateEzaFlat(data)
	case "eza-family":
		return filesValidateEzaFamily(data)
	case "eza-native":
		return filesValidateEzaNative(data)
	default:
		return fmt.Errorf("unsupported eza port format %q", port.Format)
	}
}

// Check the native upstream structure before the caller merges it into the
// active YAML. In particular, arbitrary nonempty YAML (such as lazygit's gui
// palette) is not an eza theme.
func filesValidateEzaNative(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return fmt.Errorf("parse eza native YAML: %w", err)
	}
	if colourful, ok := config["colourful"].(bool); !ok || !colourful {
		return fmt.Errorf("eza native theme requires colourful: true")
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("eza native theme requires %s styles", section)
		}
		for name, value := range styles {
			if err := filesValidateEzaNativeStyle(section+"."+name, value); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"punctuation", "date", "inode", "blocks", "header", "octal", "flags", "symlink_path", "control_char", "broken_symlink", "broken_path_overlay"} {
		if err := filesValidateEzaNativeStyle(key, config[key]); err != nil {
			return err
		}
	}
	return nil
}

func filesValidateEzaNativeStyle(name string, value any) error {
	style, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("eza native %s must be a style mapping", name)
	}
	foreground, ok := style["foreground"].(string)
	if !ok || foreground == "" {
		return fmt.Errorf("eza native %s requires a foreground color", name)
	}
	return nil
}

var filesEzaFamilyHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate the eza theme.yml style tables, rather than merely accepting YAML
// that could belong to another application.
func filesValidateEzaFamily(data []byte) error {
	config, err := shared.YAML(data)
	if err != nil {
		return fmt.Errorf("parse eza theme YAML: %w", err)
	}
	if colourful, ok := config["colourful"].(bool); !ok || !colourful {
		return fmt.Errorf("eza theme requires colourful: true")
	}
	for _, section := range []string{"filekinds", "perms", "size", "users", "links", "git", "git_repo", "security_context", "file_type"} {
		styles, ok := config[section].(map[string]any)
		if !ok || len(styles) == 0 {
			return fmt.Errorf("eza theme requires native %s styles", section)
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
		return fmt.Errorf("eza %s must be a native style mapping", name)
	}
	foreground, ok := style["foreground"].(string)
	if !ok || !filesEzaFamilyHexColor.MatchString(foreground) {
		return fmt.Errorf("eza %s requires a hexadecimal foreground color", name)
	}
	for key, value := range style {
		switch key {
		case "foreground", "background":
			color, ok := value.(string)
			if !ok || !filesEzaFamilyHexColor.MatchString(color) {
				return fmt.Errorf("eza %s.%s requires a hexadecimal color", name, key)
			}
		case "is_bold", "is_dimmed", "is_italic", "is_underline", "is_blink", "is_reverse", "is_hidden", "is_strikethrough", "prefix_with_reset":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("eza %s.%s requires a boolean", name, key)
			}
		default:
			return fmt.Errorf("eza %s.%s is not a native style property", name, key)
		}
	}
	return nil
}

func filesResolveEza(spec model.ThemePort, _ model.Environment) (model.Port, bool, error) {
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func filesPortData(port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" || port.URL != "" {
		return shared.Custom(port, env)
	}
	if port.Kind == "official" {
		if !catalog.Verified("eza", port) {
			return nil, fmt.Errorf("unregistered eza official port %q", port.Name)
		}
		data, err := shared.AcquireAsset(env, port.Source)
		if err != nil {
			return nil, fmt.Errorf("acquire eza %s: %w", port.Name, err)
		}
		if err := filesValidateEzaPort(port, data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, fmt.Errorf("eza requires an available official port or a registered native theme file")
}
