package adapter

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

const (
	filesEzaDraculaBase          = "https://raw.githubusercontent.com/eza-community/eza-themes/21184b55bfe34e8473c8160f9de0c03e4f76857f/"
	filesEzaDraculaSource        = filesEzaDraculaBase + "themes/dracula.yml"
	filesEzaDraculaSHA256        = "159356adfba85199ee2bad46b564f59033b801b1da8392b5ad5be94c180f8be2"
	filesEzaDraculaLicenseSource = filesEzaDraculaBase + "LICENSE"
	filesEzaDraculaLicenseSHA256 = "b5428ac060b6fec749a8ccaaa1f977932b77e40e096b37474a2667d91f6d80b8"
)

// Acquisition is apply-only. Keep the upstream MIT notice in the same verified
// content-addressed cache as the theme; neither asset is an embedded fallback.
func filesAcquireEzaDracula(env model.Environment) ([]byte, error) {
	data, err := filesAcquireAsset(env, filesEzaDraculaSource, filesEzaDraculaSHA256)
	if err != nil {
		return nil, fmt.Errorf("acquire eza Dracula: %w", err)
	}
	if err := filesValidateEzaDracula(data); err != nil {
		return nil, err
	}
	if _, err := filesAcquireAsset(env, filesEzaDraculaLicenseSource, filesEzaDraculaLicenseSHA256); err != nil {
		return nil, fmt.Errorf("acquire eza Dracula license: %w", err)
	}
	return data, nil
}

// Check the native upstream structure before the caller merges it into the
// active YAML. In particular, arbitrary nonempty YAML (such as lazygit's gui
// palette) is not an eza theme. The asset checksum fixes the precise key set.
func filesValidateEzaDracula(data []byte) error {
	config, err := filesYAML(data)
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
