package shared

import "github.com/clang-engineer/themeset/internal/model"

const MoonflySource = "https://raw.githubusercontent.com/bluz71/vim-moonfly-colors/master/extras/"
const RosePineYaziBase = "https://raw.githubusercontent.com/rose-pine/yazi/main/"
const RosePineYaziSource = "https://github.com/rose-pine/yazi/tree/main/flavors/rose-pine.yazi"

var RosePineYaziAssets = []string{
	"flavors/rose-pine.yazi/flavor.toml",
	"flavors/rose-pine.yazi/tmtheme.xml",
}

func RosePineSource(id string) (string, bool) {
	var repo, file string
	switch id {
	case "starship":
		repo, file = "starship", "rose-pine.toml"
	case "fzf":
		repo, file = "fzf", "dist/rose-pine.sh"
	case "lazygit":
		repo, file = "lazygit", "themes/rose-pine.yml"
	case "atuin":
		repo, file = "atuin", "dist/rose-pine.toml"
	case "eza":
		repo, file = "eza", "dist/rose-pine.yml"
	case "btop":
		repo, file = "btop", "rose-pine.theme"
	default:
		return "", false
	}
	return "https://raw.githubusercontent.com/rose-pine/" + repo + "/main/" + file, true
}

const TokyoNightBase = "https://raw.githubusercontent.com/folke/tokyonight.nvim/main/"

var tokyoNightAssets = map[string]string{
	"btop":    "extras/btop/tokyonight_night.theme",
	"eza":     "extras/eza/tokyonight_night.yml",
	"fzf":     "extras/fzf/tokyonight_night.sh",
	"lazygit": "extras/lazygit/tokyonight_night.yml",
	"yazi":    "extras/yazi/tokyonight_night.toml",
	"syntax":  "extras/sublime/tokyonight_night.tmTheme",
	"tmux":    "extras/tmux/tokyonight_night.tmux",
}

func TokyoNightSource(id string) (string, bool) {
	asset, ok := tokyoNightAssets[id]
	if !ok {
		return "", false
	}
	return TokyoNightBase + asset, true
}

func ResolveTokyoNightPort(id, name string, _ model.Environment) (model.Port, bool, error) {
	source, ok := TokyoNightSource(id)
	if name != "tokyo-night" || !ok || id == "syntax" {
		return model.Port{}, false, nil
	}
	selected := name
	if id == "btop" {
		selected = "tokyonight_night"
	}
	return model.Port{Kind: "official", Name: selected, Source: source}, true, nil
}

func ResolveRosePinePort(id, name string, _ model.Environment) (model.Port, bool, error) {
	if name != "rose-pine" {
		return model.Port{}, false, nil
	}
	source, ok := RosePineSource(id)
	if !ok {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: source}, true, nil
}

func OfficialPort(id, name string) (model.Port, bool) {
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

const YaziDraculaSource = "https://github.com/yazi-rs/flavors/tree/main/dracula.yazi"
const YaziDraculaRaw = "https://raw.githubusercontent.com/yazi-rs/flavors/main/dracula.yazi/"

var YaziDraculaAssets = []string{"flavor.toml", "tmtheme.xml", "LICENSE", "LICENSE-tmtheme"}

const EzaDraculaBase = "https://raw.githubusercontent.com/eza-community/eza-themes/main/"
const EzaDraculaSource = EzaDraculaBase + "themes/dracula.yml"
const EzaDraculaLicenseSource = EzaDraculaBase + "LICENSE"

const BtopCatppuccinBase = "https://raw.githubusercontent.com/catppuccin/btop/main/"
const BtopCatppuccinSource = BtopCatppuccinBase + "themes/catppuccin_mocha.theme"
const BtopCatppuccinLicense = BtopCatppuccinBase + "LICENSE"
