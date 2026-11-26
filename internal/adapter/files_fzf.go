package adapter

import (
	_ "embed"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// Exact existing upstream assets; the sibling LICENSE files record revisions.
//
//go:embed assets/fzf-dracula.opts
var filesFZFDracula []byte

//go:embed assets/fzf-catppuccin-mocha.opts
var filesFZFCatppuccinMocha []byte

//go:embed assets/fzf-gruvbox.opts
var filesFZFGruvbox []byte

//go:embed assets/fzf-nord.opts
var filesFZFNord []byte

//go:embed assets/fzf-moonfly.opts
var filesFZFMoonfly []byte

const filesFZFSource = "https://github.com/dracula/fzf/tree/f4c7758a813ca73b69e9c954c001586c33858c24"
const filesFZFCatppuccinSource = "https://github.com/catppuccin/fzf/blob/7508f8141286fb95249100a2b5325960320dcf32/themes/catppuccin-fzf-mocha.rc"
const filesFZFPresetsSource = "https://github.com/junegunn/junegunn.github.io/blob/8112383a1781605e7b181c5b717327439a0fb981/static/js/fzf-color-themes.js"

func filesResolveFZF(name string, env model.Environment) (model.Port, bool, error) {
	if port, found, err := filesResolveTokyoNightPort("fzf", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveRosePinePort("fzf", name, env); found || err != nil {
		return port, found, err
	}
	switch name {
	case "catppuccin-mocha":
		if err := filesFZFRequireSelectedColors(env); err != nil {
			return model.Port{}, false, err
		}
		return model.Port{Kind: "official", Name: name, Source: filesFZFCatppuccinSource}, true, nil
	case "gruvbox", "nord":
		return model.Port{Kind: "official", Name: name, Source: filesFZFPresetsSource}, true, nil
	case "dracula":
		return model.Port{Kind: "official", Name: name, Source: filesFZFSource}, true, nil
	case "moonfly":
		return model.Port{Kind: "official", Name: name, Source: filesMoonflySource + "moonfly-fzf.sh"}, true, nil
	default:
		return model.Port{}, false, nil
	}
}

// Custom fzf ports contain one --color=... option per line, not shell code or
// executable --bind/--preview options. This file is appended by shell integration
// rather than replacing the user's ordinary FZF_DEFAULT_OPTS(_FILE).
func filesFZFOptions(data []byte) error {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "--color=") || len(line) == len("--color=") || strings.ContainsAny(line, " \t\r\x00\"'`$") {
			return errors.New("fzf theme files require one unquoted --color=... option per line; other options and shell code are not accepted")
		}
		count++
	}
	if count == 0 {
		return errors.New("fzf theme file contains no --color options")
	}
	return nil
}

var filesFZFVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:\s|$)`)

func filesFZFSelectedColors(version []byte) bool {
	text := strings.TrimSpace(string(version))
	if !filesFZFVersion.MatchString(text) {
		return false
	}
	var major, minor, patch int
	if n, err := fmt.Sscanf(text, "%d.%d.%d", &major, &minor, &patch); err != nil || n != 3 {
		return false
	}
	return major > 0 || minor >= 52
}

func filesFZFRequireSelectedColors(env model.Environment) error {
	version, err := nativeCommand(env, "fzf", "--version")
	if err != nil {
		return fmt.Errorf("Catppuccin Mocha requires fzf 0.52.0 or newer: %w", err)
	}
	if !filesFZFSelectedColors(version) {
		return errors.New("Catppuccin Mocha requires fzf 0.52.0 or newer")
	}
	return nil
}

func filesApplyFZF(port model.Port, env model.Environment) error {
	var data []byte
	if port.File != "" {
		var err error
		data, err = filesCustom(port, env)
		if err != nil {
			return err
		}
	} else if port.Kind == "official" {
		switch port.Name {
		case "catppuccin-mocha":
			// The exact theme-author port uses selected-bg, added in 0.52.0.
			if err := filesFZFRequireSelectedColors(env); err != nil {
				return err
			}
			data = filesFZFCatppuccinMocha
		case "gruvbox":
			data = filesFZFGruvbox
		case "nord":
			data = filesFZFNord
		case "dracula":
			data = filesFZFDracula
		case "moonfly":
			data = filesFZFMoonfly
		case "tokyo-night":
			var err error
			data, err = filesAcquireTokyoNightPort("fzf", port, env)
			if err != nil {
				return err
			}
		case "rose-pine":
			var err error
			data, err = filesAcquireRosePinePort("fzf", port, env)
			if err != nil {
				return err
			}
		default:
			return errors.New("unknown official fzf port")
		}
	} else {
		return errors.New("fzf requires an official port or custom options file")
	}
	if err := filesFZFOptions(data); err != nil {
		return err
	}
	path := filesPath("fzf", env)
	existing, err := filesRead(path, true)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if err := filesFZFOptions(existing); err != nil {
			return fmt.Errorf("refusing to replace non-theme options in %s: %w", path, err)
		}
	}
	var options strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			options.WriteString(line)
			options.WriteByte('\n')
		}
	}
	return filesAtomic(path, []byte(options.String()))
}
