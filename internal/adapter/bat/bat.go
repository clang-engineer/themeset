package bat

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

var names = map[string][]string{
	"dracula": {"Dracula"}, "catppuccin-mocha": {"Catppuccin Mocha"}, "nord": {"Nord"},
	"rose-pine": {"Rosé Pine", "Rose Pine", "rose-pine"},
	"gruvbox":   {"gruvbox-dark"}, "gruvbox-dark": {"gruvbox-dark"},
	"tokyonight": {"tokyonight_night"}, "tokyo-night": {"tokyonight_night"},
}

func Adapter() model.Adapter {
	listed := shared.ListedResolver("bat", names, "https://github.com/sharkdp/bat/tree/master/assets/themes", "--list-themes", "--color=never")
	return model.Adapter{ID: "bat", Binary: "bat", Resolve: func(theme string, env model.Environment) (model.Port, bool, error) {
		port, ok, err := listed(theme, env)
		if ok && shared.ThemeKey(theme) == "rose-pine" {
			port.Source = "https://github.com/rose-pine/tm-theme"
		}
		return port, ok, err
	}, Apply: nativeApplyBat}
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
	name, err := shared.Selection(port, env, nativeParseBat)
	if err != nil {
		return err
	}
	path := shared.NativeExpand(env.Paths["bat"], env)
	if path == "" {
		out, err := shared.Command(env, "bat", "--config-file")
		if err != nil {
			return err
		}
		path = strings.TrimSpace(string(out))
		if !filepath.IsAbs(path) {
			return fmt.Errorf("bat returned a non-absolute native config path %q", path)
		}
	}
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	return shared.Atomic(path, shared.ReplaceLines(data, func(s string) bool { return nativeBatTheme.MatchString(s) }, "--theme="+strconv.Quote(name)))
}
