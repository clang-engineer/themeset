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

const EzaDraculaBase = EzaFamilyBase
const EzaDraculaSource = EzaDraculaBase + "themes/dracula.yml"
const EzaDraculaLicenseSource = EzaDraculaBase + "LICENSE"

const BtopCatppuccinBase = "https://raw.githubusercontent.com/catppuccin/btop/main/"
const BtopCatppuccinSource = BtopCatppuccinBase + "themes/catppuccin_mocha.theme"
const BtopCatppuccinLicense = BtopCatppuccinBase + "LICENSE"

// Source provenance lives here; each tool adapter still owns format validation
// and the decision to use a registered source for a requested theme.
const (
	AtuinThemesSource          = "https://docs.atuin.sh/latest/guide/theming/"
	BatThemesSource            = "https://github.com/sharkdp/bat/tree/master/assets/themes"
	BatRosePineSource          = "https://github.com/rose-pine/tm-theme"
	BtopThemesSource           = "https://github.com/aristocratos/btop#themes"
	BtopUpstreamBase           = "https://raw.githubusercontent.com/aristocratos/btop/master/"
	BtopUpstreamLicense        = BtopUpstreamBase + "LICENSE"
	DeltaRosePineSource        = "https://raw.githubusercontent.com/rose-pine/delta/main/dist/rose-pine.gitconfig"
	DeltaThemesSource          = "https://dandavison.github.io/delta/syntax-highlighting.html"
	EzaFamilyBase              = "https://raw.githubusercontent.com/eza-community/eza-themes/main/"
	EzaGruvboxSource           = EzaFamilyBase + "themes/gruvbox-dark.yml"
	EzaFamilyLicenseSource     = EzaFamilyBase + "LICENSE"
	FZFDraculaSource           = "https://raw.githubusercontent.com/dracula/fzf/master/INSTALL.md"
	FZFCatppuccinSource        = "https://raw.githubusercontent.com/catppuccin/fzf/main/themes/catppuccin-fzf-mocha.rc"
	FZFPresetsSource           = "https://raw.githubusercontent.com/junegunn/junegunn.github.io/master/static/js/fzf-color-themes.js"
	GhosttyThemesSource        = "https://github.com/mbadolato/iTerm2-Color-Schemes/tree/master/ghostty"
	HarlequinThemesSource      = "https://harlequin.sh/docs/themes"
	StarshipCatppuccinBase     = "https://raw.githubusercontent.com/catppuccin/starship/main/"
	StarshipCatppuccinSource   = StarshipCatppuccinBase + "themes/mocha.toml"
	StarshipCatppuccinLicense  = StarshipCatppuccinBase + "LICENSE"
	StarshipGruvboxBase        = "https://raw.githubusercontent.com/starship/starship/master/"
	StarshipGruvboxSource      = StarshipGruvboxBase + "docs/public/presets/toml/gruvbox-rainbow.toml"
	StarshipGruvboxLicense     = StarshipGruvboxBase + "LICENSE"
	StarshipDraculaSource      = "https://raw.githubusercontent.com/dracula/starship/master/starship.toml"
	StarshipPalettesSource     = "https://starship.rs/config/#color-palette"
	YaziThemesSource           = "https://yazi-rs.github.io/docs/flavors/overview/"
	YaziCatppuccinSyntaxSource = "https://raw.githubusercontent.com/catppuccin/bat/main/themes/Catppuccin%20Mocha.tmTheme"
)

var neovimProviderSources = map[string]string{
	"dracula":                   "https://github.com/dracula/vim",
	"catppuccin-mocha":          "https://github.com/catppuccin/nvim",
	"nord":                      "https://github.com/nordtheme/vim",
	"gruvbox":                   "https://github.com/ellisonleao/gruvbox.nvim",
	"gruvbox-dark":              "https://github.com/ellisonleao/gruvbox.nvim",
	"tokyonight":                "https://github.com/folke/tokyonight.nvim",
	"tokyo-night":               "https://github.com/folke/tokyonight.nvim",
	"rose-pine":                 "https://github.com/rose-pine/neovim",
	"kanagawa":                  "https://github.com/rebelot/kanagawa.nvim",
	"everforest":                "https://github.com/sainnhe/everforest",
	"onedark":                   "https://github.com/navarasu/onedark.nvim",
	"github-dark":               "https://github.com/projekt0n/github-nvim-theme",
	"github-dark-high-contrast": "https://github.com/projekt0n/github-nvim-theme",
	"moonfly":                   "https://github.com/bluz71/vim-moonfly-colors",
	"vscode-dark":               "https://github.com/Mofiqul/vscode.nvim",
	"sonokai":                   "https://github.com/sainnhe/sonokai",
	"cyberdream":                "https://github.com/scottmckendry/cyberdream.nvim",
	"modus-vivendi":             "https://github.com/miikanissi/modus-themes.nvim",
}

func NeovimProviderSource(theme string) string {
	return neovimProviderSources[theme]
}
