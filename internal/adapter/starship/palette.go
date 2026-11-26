package starship

import (
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

// Starship's Gruvbox Rainbow preset is maintained by Starship itself. Its
// prompt layout and module settings are never imported: only its native palette.
// Catppuccin's Mocha file is a palette-only official port.

// Return Starship TOML containing only the selected native palette and its
// selector. Neither an upstream complete preset nor other palettes can replace
// the user's prompt or module configuration. Validation makes no config writes.
func filesAcquirePaletteFamily(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire %s Starship palette: %w", port.Name, err)
	}
	data, err = filesPaletteFamilyTOML(data, port.Variant)
	if err != nil {
		return nil, fmt.Errorf("validate %s Starship palette: %w", port.Name, err)
	}
	if _, err := shared.AcquireAsset(env, port.LicenseURL); err != nil {
		return nil, fmt.Errorf("acquire %s Starship license: %w", port.Name, err)
	}
	return data, nil
}

func filesPaletteFamilyTOML(data []byte, native string) ([]byte, error) {
	config, err := shared.TOML(data)
	if err != nil {
		return nil, err
	}
	if selected, ok := config["palette"]; ok && selected != native {
		return nil, fmt.Errorf("unexpected Starship palette selector %v", selected)
	}
	palettes, ok := config["palettes"].(map[string]any)
	if !ok {
		return nil, errors.New("Starship palette tables missing")
	}
	palette, ok := palettes[native].(map[string]any)
	if !ok || len(palette) == 0 {
		return nil, fmt.Errorf("Starship palette %q is empty or missing", native)
	}
	return toml.Marshal(map[string]any{
		"palette":  native,
		"palettes": map[string]any{native: palette},
	})
}
