package adapter

import (
	"fmt"
	"regexp"

	"github.com/clang-engineer/themeset/internal/model"
)

// The eza-maintained Gruvbox Dark port (not a Gruvbox-author port). The
// eza-themes MIT license is acquired into the same verified cache as the YAML.
const (
	filesEzaFamilyBase          = "https://raw.githubusercontent.com/eza-community/eza-themes/21184b55bfe34e8473c8160f9de0c03e4f76857f/"
	filesEzaGruvboxSource       = filesEzaFamilyBase + "themes/gruvbox-dark.yml"
	filesEzaGruvboxSHA256       = "eafc73490c369aaf2887412cd4b6266db6afb8dd0b14e8380a1eaed46f5dbf8c"
	filesEzaFamilyLicenseSource = filesEzaFamilyBase + "LICENSE"
	filesEzaFamilyLicenseSHA256 = "b5428ac060b6fec749a8ccaaa1f977932b77e40e096b37474a2667d91f6d80b8"
)

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
	data, err := filesAcquireAsset(env, filesEzaGruvboxSource, filesEzaGruvboxSHA256)
	if err != nil {
		return nil, fmt.Errorf("acquire eza Gruvbox Dark: %w", err)
	}
	if err := filesValidateEzaFamily(data); err != nil {
		return nil, err
	}
	if _, err := filesAcquireAsset(env, filesEzaFamilyLicenseSource, filesEzaFamilyLicenseSHA256); err != nil {
		return nil, fmt.Errorf("acquire eza Gruvbox Dark license: %w", err)
	}
	return data, nil
}

var filesEzaFamilyHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Validate the eza theme.yml style tables, rather than merely accepting YAML
// that could belong to another application. A pinned checksum fixes the exact
// selection of colors and keys; this check catches wrong-schema cache contents.
func filesValidateEzaFamily(data []byte) error {
	config, err := filesYAML(data)
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
