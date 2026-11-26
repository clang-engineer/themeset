package adapter

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesResolveRosePinePort(id, name string, _ model.Environment) (model.Port, bool, error) {
	if name != "rose-pine" {
		return model.Port{}, false, nil
	}
	recipe, ok := rosePineRecipe(id)
	if !ok {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: recipe.url}, true, nil
}

func filesAcquireRosePinePort(id string, port model.Port, env model.Environment) ([]byte, error) {
	recipe, ok := rosePineRecipe(id)
	if !ok || port.Kind != "official" || port.Name != "rose-pine" || port.Source != recipe.url || port.File != "" {
		return nil, fmt.Errorf("unregistered Rosé Pine %s port", id)
	}
	data, err := filesAcquireAsset(env, recipe.url)
	if err != nil {
		return nil, fmt.Errorf("acquire Rosé Pine %s theme: %w", id, err)
	}
	data, err = filesPrepareOfficialPort(id, "rose-pine", data)
	if err != nil {
		return nil, fmt.Errorf("invalid Rosé Pine %s theme: %w", id, err)
	}
	return data, nil
}
func filesResolveTokyoNightPort(id, name string, _ model.Environment) (model.Port, bool, error) {
	asset, ok := filesTokyoNightAssets[id]
	if name != "tokyo-night" || !ok || id == "syntax" {
		return model.Port{}, false, nil
	}
	selected := name
	if id == "btop" {
		selected = "tokyonight_night"
	}
	return model.Port{Kind: "official", Name: selected, Source: filesTokyoNightBase + asset}, true, nil
}

// Only a registered source may enter acquisition; resolution is always read-only.
func filesAcquireTokyoNightPort(id string, port model.Port, env model.Environment) ([]byte, error) {
	asset, ok := filesTokyoNightAssets[id]
	if !ok || id == "syntax" || id == "tmux" || port.Kind != "official" || port.File != "" || port.Source != filesTokyoNightBase+asset || id == "btop" && port.Name != "tokyonight_night" || id != "btop" && port.Name != "tokyo-night" {
		return nil, fmt.Errorf("unregistered Tokyo Night %s port", id)
	}
	data, err := filesAcquireAsset(env, port.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night %s: %w", id, err)
	}
	data, err = filesPrepareOfficialPort(id, "tokyo-night", data)
	if err != nil {
		return nil, fmt.Errorf("invalid Tokyo Night %s port: %w", id, err)
	}
	return data, nil
}

// Parsing and validation live with each tool; theme records only select sources.
func filesPrepareOfficialPort(id, theme string, data []byte) ([]byte, error) {
	switch id {
	case "starship":
		return filesPrepareStarshipPort(theme, data)
	case "fzf":
		return filesPrepareFZFPort(theme, data)
	case "lazygit":
		return data, filesValidateLazyPort(data)
	case "atuin":
		return data, filesValidateAtuinPort(theme, data)
	case "eza":
		return data, filesValidateEzaPort(theme, data)
	case "btop":
		return data, filesBtopTheme(data)
	case "yazi":
		return data, filesValidateYaziPort(data)
	default:
		return nil, fmt.Errorf("unsupported official port tool %q", id)
	}
}
