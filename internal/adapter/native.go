package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

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
		"gruvbox": {"gruvbox-dark"}, "gruvbox-dark": {"gruvbox-dark"},
		"tokyonight": {"tokyonight_night"}, "tokyo-night": {"tokyonight_night"},
	},
	"delta": {
		"dracula": {"Dracula"}, "catppuccin-mocha": {"Catppuccin Mocha"}, "nord": {"Nord"},
		"gruvbox": {"gruvbox-dark"}, "gruvbox-dark": {"gruvbox-dark"}, "tokyonight": {"tokyonight_night"}, "tokyo-night": {"tokyonight_night"},
	},
	"harlequin": {
		"dracula": {"dracula"}, "catppuccin-mocha": {"catppuccin-mocha"}, "nord": {"nord"}, "gruvbox": {"gruvbox"},
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
		{ID: "delta", Binary: "delta", Resolve: nativeListedResolver("delta", "--list-syntax-themes"), Apply: nativeApplyDelta},
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
				return model.Port{Kind: "native", Name: name, Source: nativeSources[id]}, true, nil
			}
		}
		return model.Port{}, false, nil
	}
}

func nativeGhosttyPath(env model.Environment) string {
	xdg := filepath.Join(nativeConfigHome(env), "ghostty", "config")
	if env.OS == "darwin" {
		mac := filepath.Join(env.Home, "Library", "Application Support", "com.mitchellh.ghostty", "config")
		if _, err := os.Stat(mac); err == nil {
			xdg = mac
		}
	}
	return nativePath("ghostty", env, xdg)
}

var nativeGhosttyTheme = regexp.MustCompile(`^\s*theme\s*=`)

func nativeApplyGhostty(port model.Port, env model.Environment) error {
	name := port.Name
	if port.File != "" {
		name = nativeExpand(port.File, env)
		info, err := os.Stat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Ghostty theme must be a regular theme asset: %s", name)
		}
	}
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("invalid Ghostty theme name")
	}
	path := nativeGhosttyPath(env)
	if port.File != "" && filepath.Clean(nativeExpand(port.File, env)) == filepath.Clean(path) {
		return fmt.Errorf("Ghostty theme asset cannot be the destination config")
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	if err := nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return nativeGhosttyTheme.MatchString(s) }, "theme = "+strconv.Quote(name))); err != nil {
		return err
	}
	return nativeReloadGhostty(env)
}

var nativeBatTheme = regexp.MustCompile(`^\s*--theme(?:\s*=|\s+|$)`)

func nativeParseBat(data []byte) (string, error) {
	name := ""
	for _, line := range strings.Split(string(data), "\n") {
		if !nativeBatTheme.MatchString(line) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "--theme"))
		value = strings.TrimSpace(strings.TrimPrefix(value, "="))
		if strings.HasPrefix(value, "\"") {
			parsed, err := strconv.Unquote(value)
			if err != nil {
				return "", err
			}
			value = parsed
		} else if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			value = strings.Trim(value, "'")
		}
		if value == "" || strings.ContainsAny(value, "\r\n\x00") {
			return "", fmt.Errorf("invalid bat theme selection")
		}
		name = value
	}
	if name == "" {
		return "", fmt.Errorf("custom bat config contains no --theme selection")
	}
	return name, nil
}

func nativeApplyBat(port model.Port, env model.Environment) error {
	name, err := nativeSelection(port, env, nativeParseBat)
	if err != nil {
		return err
	}
	path := nativeExpand(env.Paths["bat"], env)
	if path == "" {
		out, err := nativeCommand(env, "bat", "--config-file")
		if err != nil {
			return err
		}
		path = strings.TrimSpace(string(out))
		if !filepath.IsAbs(path) {
			return fmt.Errorf("bat returned a non-absolute native config path %q", path)
		}
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	return nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return nativeBatTheme.MatchString(s) }, "--theme="+strconv.Quote(name)))
}
