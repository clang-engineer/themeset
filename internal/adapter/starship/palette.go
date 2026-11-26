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
const (
	filesPaletteCatBase           = "https://raw.githubusercontent.com/catppuccin/starship/main/"
	filesPaletteCatSource         = filesPaletteCatBase + "themes/mocha.toml"
	filesPaletteCatLicenseSource  = filesPaletteCatBase + "LICENSE"
	filesPaletteGruvBase          = "https://raw.githubusercontent.com/starship/starship/master/"
	filesPaletteGruvSource        = filesPaletteGruvBase + "docs/public/presets/toml/gruvbox-rainbow.toml"
	filesPaletteGruvLicenseSource = filesPaletteGruvBase + "LICENSE"
)

type filesPaletteRecipe struct {
	source, license, native string
}

func filesPaletteFamilyRecipe(id, name string) (filesPaletteRecipe, bool) {
	if id != "starship" {
		return filesPaletteRecipe{}, false
	}
	switch name {
	case "catppuccin-mocha":
		return filesPaletteRecipe{filesPaletteCatSource, filesPaletteCatLicenseSource, "catppuccin_mocha"}, true
	case "gruvbox":
		return filesPaletteRecipe{filesPaletteGruvSource, filesPaletteGruvLicenseSource, "gruvbox_dark"}, true
	default:
		return filesPaletteRecipe{}, false
	}
}

// Resolve is read-only, including in a dry-run. The caller checks registered
// custom ports first and may fall back to an already defined native palette.
func filesResolvePaletteFamily(id, name string, _ model.Environment) (model.Port, bool, error) {
	recipe, ok := filesPaletteFamilyRecipe(id, name)
	if !ok {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: recipe.source}, true, nil
}

// Return Starship TOML containing only the selected native palette and its
// selector. Neither an upstream complete preset nor other palettes can replace
// the user's prompt or module configuration. Validation makes no config writes.
func filesAcquirePaletteFamily(id, name string, env model.Environment) ([]byte, error) {
	recipe, ok := filesPaletteFamilyRecipe(id, name)
	if !ok {
		return nil, fmt.Errorf("no registered %s palette-family port for %s", id, name)
	}
	data, err := shared.AcquireAsset(env, recipe.source)
	if err != nil {
		return nil, fmt.Errorf("acquire %s %s palette: %w", id, name, err)
	}
	data, err = filesPaletteFamilyTOML(data, recipe.native)
	if err != nil {
		return nil, fmt.Errorf("validate %s %s palette: %w", id, name, err)
	}
	if _, err := shared.AcquireAsset(env, recipe.license); err != nil {
		return nil, fmt.Errorf("acquire %s %s license: %w", id, name, err)
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
