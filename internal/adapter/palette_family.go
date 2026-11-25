package adapter

import (
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

// Starship's Gruvbox Rainbow preset is maintained by Starship itself. Its
// prompt layout and module settings are never imported: only its native palette.
// Catppuccin's Mocha file is a palette-only official port.
const (
	filesPaletteCatSource         = "https://raw.githubusercontent.com/catppuccin/starship/5906cc369dd8207e063c0e6e2d27bd0c0b567cb8/themes/mocha.toml"
	filesPaletteCatSHA256         = "712699ad27db93deddda4577ab9753f76e0c82652e63c7982085343c8ca01f9c"
	filesPaletteCatLicenseSource  = "https://raw.githubusercontent.com/catppuccin/starship/5906cc369dd8207e063c0e6e2d27bd0c0b567cb8/LICENSE"
	filesPaletteCatLicenseSHA256  = "814096d2c34cc216c624738a49356f32b7237733b4f7edb0685f4e50ef5074ba"
	filesPaletteGruvSource        = "https://raw.githubusercontent.com/starship/starship/1b5c2f257ef276c3020899623a1942b3ecd457f7/docs/public/presets/toml/gruvbox-rainbow.toml"
	filesPaletteGruvSHA256        = "61e960a1bcd0cd1aa05527b5c4de1bf0ad9911c1e97c96cb3e58a56aad8434de"
	filesPaletteGruvLicenseSource = "https://raw.githubusercontent.com/starship/starship/1b5c2f257ef276c3020899623a1942b3ecd457f7/LICENSE"
	filesPaletteGruvLicenseSHA256 = "2587580d0ec4ddc43d1534745a19ffeec0b63186497c2ed4bc36832da91b13e2"
)

type filesPaletteRecipe struct {
	source, digest, license, licenseDigest, native string
}

func filesPaletteFamilyRecipe(id, name string) (filesPaletteRecipe, bool) {
	if id != "starship" {
		return filesPaletteRecipe{}, false
	}
	switch name {
	case "catppuccin-mocha":
		return filesPaletteRecipe{filesPaletteCatSource, filesPaletteCatSHA256, filesPaletteCatLicenseSource, filesPaletteCatLicenseSHA256, "catppuccin_mocha"}, true
	case "gruvbox":
		return filesPaletteRecipe{filesPaletteGruvSource, filesPaletteGruvSHA256, filesPaletteGruvLicenseSource, filesPaletteGruvLicenseSHA256, "gruvbox_dark"}, true
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
		return nil, fmt.Errorf("no verified %s palette-family port for %s", id, name)
	}
	data, err := filesAcquireAsset(env, recipe.source, recipe.digest)
	if err != nil {
		return nil, fmt.Errorf("acquire %s %s palette: %w", id, name, err)
	}
	data, err = filesPaletteFamilyTOML(data, recipe.native)
	if err != nil {
		return nil, fmt.Errorf("validate %s %s palette: %w", id, name, err)
	}
	if _, err := filesAcquireAsset(env, recipe.license, recipe.licenseDigest); err != nil {
		return nil, fmt.Errorf("acquire %s %s license: %w", id, name, err)
	}
	return data, nil
}

func filesPaletteFamilyTOML(data []byte, native string) ([]byte, error) {
	config, err := filesTOML(data)
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
