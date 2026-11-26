package adapter

import "github.com/clang-engineer/themeset/internal/model"

// These are upstream theme names, never palette approximations. Availability is
// confirmed against each installed application's list before returning a port.
var nativeNames = map[string]map[string][]string{
	"ghostty": {
		"dracula": {"Dracula"}, "catppuccin-mocha": {"Catppuccin Mocha"}, "nord": {"Nord"}, "gruvbox": {"Gruvbox Dark"},
		"gruvbox-dark": {"Gruvbox Dark"}, "gruvbox-dark-hard": {"Gruvbox Dark Hard"},
		"tokyonight": {"TokyoNight Night"}, "tokyo-night": {"TokyoNight Night"}, "rose-pine": {"Rose Pine"}, "kanagawa": {"Kanagawa Wave"},
		"everforest": {"Everforest Dark Med", "everforest"}, "github-dark": {"GitHub Dark"}, "moonfly": {"Moonfly", "moonfly"},
		"sonokai": {"Sonokai"}, "cyberdream": {"cyberdream"}, "onedark": {"onedark", "One Dark"},
		"github-dark-high-contrast": {"GitHub Dark High Contrast"}, "gruvbox-hard": {"Gruvbox Dark Hard"},
		"modus-vivendi": {"Modus Vivendi"}, "vscode-dark": {"vscode-dark"},
	},
	"bat": {
		"dracula": {"Dracula"}, "catppuccin-mocha": {"Catppuccin Mocha"}, "nord": {"Nord"},
		"rose-pine": {"Rosé Pine", "Rose Pine", "rose-pine"},
		"gruvbox":   {"gruvbox-dark"}, "gruvbox-dark": {"gruvbox-dark"},
		"tokyonight": {"tokyonight_night"}, "tokyo-night": {"tokyonight_night"},
	},
	"delta": {
		"dracula": {"Dracula"}, "catppuccin-mocha": {"Catppuccin Mocha"}, "nord": {"Nord"},
		"gruvbox": {"gruvbox-dark"}, "gruvbox-dark": {"gruvbox-dark"}, "tokyonight": {"tokyonight_night"}, "tokyo-night": {"tokyonight_night"},
	},
	"harlequin": {
		"dracula": {"dracula"}, "catppuccin-mocha": {"catppuccin-mocha"}, "nord": {"nord"}, "gruvbox": {"gruvbox"},
		"rose-pine":  {"rose-pine"},
		"tokyonight": {"tokyo-night"}, "tokyo-night": {"tokyo-night"}, "monokai": {"monokai"}, "solarized-light": {"solarized-light"},
	},
}

var nativeSources = map[string]string{
	"ghostty":   "https://github.com/mbadolato/iTerm2-Color-Schemes/tree/master/ghostty",
	"bat":       "https://github.com/sharkdp/bat/tree/master/assets/themes",
	"delta":     "https://dandavison.github.io/delta/syntax-highlighting.html",
	"harlequin": "https://harlequin.sh/docs/themes",
}

func nativeAdapters() []model.Adapter {
	return []model.Adapter{
		{ID: "ghostty", Binary: "ghostty", Resolve: nativeListedResolver("ghostty", "+list-themes"), Apply: nativeApplyGhostty},
		{ID: "neovim", Binary: "nvim", Resolve: nativeResolveNeovim, Apply: nativeApplyNeovim},
		{ID: "tmux", Binary: "tmux", Resolve: nativeResolveTmux, Apply: nativeApplyTmux},
		{ID: "bat", Binary: "bat", Resolve: nativeListedResolver("bat", "--list-themes", "--color=never"), Apply: nativeApplyBat},
		{ID: "delta", Binary: "delta", Resolve: nativeResolveDelta, Apply: nativeApplyDelta},
		{ID: "harlequin", Binary: "harlequin", Resolve: nativeListedResolver("harlequin", "--help"), Apply: nativeApplyHarlequin},
	}
}

func nativeListedResolver(id string, args ...string) func(string, model.Environment) (model.Port, bool, error) {
	return func(theme string, env model.Environment) (model.Port, bool, error) {
		names := nativeNames[id][nativeThemeKey(theme)]
		if len(names) == 0 {
			return model.Port{}, false, nil
		}
		out, err := nativeCommand(env, id, args...)
		if err != nil {
			return model.Port{}, false, err
		}
		for _, name := range names {
			available := nativeListContains(out, name)
			if id == "harlequin" {
				available = nativeHarlequinHasTheme(out, name)
			}
			if available {
				source := nativeSources[id]
				if id == "bat" && nativeThemeKey(theme) == "rose-pine" {
					source = "https://github.com/rose-pine/tm-theme"
				}
				return model.Port{Kind: "native", Name: name, Source: source}, true, nil
			}
		}
		return model.Port{}, false, nil
	}
}
