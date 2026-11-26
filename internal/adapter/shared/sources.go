package shared

const MoonflySource = "https://raw.githubusercontent.com/bluz71/vim-moonfly-colors/master/extras/"
const RosePineYaziBase = "https://raw.githubusercontent.com/rose-pine/yazi/main/"
const RosePineYaziSource = "https://github.com/rose-pine/yazi/tree/main/flavors/rose-pine.yazi"

var RosePineYaziAssets = []string{
	"flavors/rose-pine.yazi/flavor.toml",
	"flavors/rose-pine.yazi/tmtheme.xml",
}

const YaziDraculaSource = "https://github.com/yazi-rs/flavors/tree/main/dracula.yazi"
const YaziDraculaRaw = "https://raw.githubusercontent.com/yazi-rs/flavors/main/dracula.yazi/"

var YaziDraculaAssets = []string{"flavor.toml", "tmtheme.xml", "LICENSE", "LICENSE-tmtheme"}

const EzaDraculaBase = EzaFamilyBase
const EzaDraculaSource = EzaDraculaBase + "themes/dracula.yml"

const BtopCatppuccinBase = "https://raw.githubusercontent.com/catppuccin/btop/main/"
const BtopCatppuccinSource = BtopCatppuccinBase + "themes/catppuccin_mocha.theme"

// Tool-native provider provenance and legacy test fixture URLs live here.
// Official port sources are selected exclusively from the bundled catalog.
const (
	AtuinThemesSource        = "https://docs.atuin.sh/latest/guide/theming/"
	BatThemesSource          = "https://github.com/sharkdp/bat/tree/master/assets/themes"
	BtopThemesSource         = "https://github.com/aristocratos/btop#themes"
	DeltaThemesSource        = "https://dandavison.github.io/delta/syntax-highlighting.html"
	EzaFamilyBase            = "https://raw.githubusercontent.com/eza-community/eza-themes/main/"
	EzaGruvboxSource         = EzaFamilyBase + "themes/gruvbox-dark.yml"
	FZFDraculaSource         = "https://raw.githubusercontent.com/dracula/fzf/master/INSTALL.md"
	FZFCatppuccinSource      = "https://raw.githubusercontent.com/catppuccin/fzf/main/themes/catppuccin-fzf-mocha.rc"
	FZFPresetsSource         = "https://raw.githubusercontent.com/junegunn/junegunn.github.io/master/static/js/fzf-color-themes.js"
	GhosttyThemesSource      = "https://github.com/mbadolato/iTerm2-Color-Schemes/tree/master/ghostty"
	HarlequinThemesSource    = "https://harlequin.sh/docs/themes"
	StarshipCatppuccinBase   = "https://raw.githubusercontent.com/catppuccin/starship/main/"
	StarshipCatppuccinSource = StarshipCatppuccinBase + "themes/mocha.toml"
	StarshipGruvboxBase      = "https://raw.githubusercontent.com/starship/starship/master/"
	StarshipGruvboxSource    = StarshipGruvboxBase + "docs/public/presets/toml/gruvbox-rainbow.toml"
	StarshipDraculaSource    = "https://raw.githubusercontent.com/dracula/starship/master/starship.toml"
	StarshipPalettesSource   = "https://starship.rs/config/#color-palette"
	YaziThemesSource         = "https://yazi-rs.github.io/docs/flavors/overview/"
)
