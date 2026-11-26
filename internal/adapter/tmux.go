package adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// Only style commands from the current author source are imported; full tmux
// theme scripts can overwrite the user's status layout or run shell commands.

func nativeResolveTmux(name string, _ model.Environment) (model.Port, bool, error) {
	switch name {
	case "moonfly":
		return model.Port{Kind: "official", Name: name, Source: filesMoonflySource + "moonfly.tmux"}, true, nil
	case "tokyo-night":
		return model.Port{Kind: "official", Name: name, Source: filesTokyoNightBase + filesTokyoNightAssets["tmux"]}, true, nil
	}
	return model.Port{}, false, nil
}

func nativeApplyTmux(port model.Port, env model.Environment) error {
	moonfly := port.File == "" && port.Kind == "official" && port.Name == "moonfly" && port.Source == filesMoonflySource+"moonfly.tmux"
	tokyo := port.File == "" && port.Kind == "official" && port.Name == "tokyo-night" && port.Source == filesTokyoNightBase+filesTokyoNightAssets["tmux"]
	if port.File == "" && !moonfly && !tokyo {
		return fmt.Errorf("tmux requires an official port or a registered tmux theme config file")
	}
	source := nativeExpand(port.File, env)
	if moonfly || tokyo {
		source = filepath.Join(nativeConfigHome(env), "themeset", "tmux-"+port.Name+".conf")
	}
	if !filepath.IsAbs(source) || strings.ContainsAny(source, "\r\n\x00") {
		return fmt.Errorf("tmux theme config requires an absolute file path")
	}
	if !moonfly && !tokyo {
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("tmux theme config must be a regular file")
		}
	}
	fallback := filepath.Join(env.Home, ".tmux.conf")
	xdg := filepath.Join(nativeConfigHome(env), "tmux", "tmux.conf")
	if _, err := os.Stat(fallback); os.IsNotExist(err) {
		if _, err := os.Stat(xdg); err == nil {
			fallback = xdg
		}
	}
	path := nativePath("tmux", env, fallback)
	if filepath.Clean(path) == filepath.Clean(source) {
		return fmt.Errorf("tmux theme source cannot be the destination config")
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	if moonfly {
		styles, err := nativeAcquireMoonflyTmux(env)
		if err != nil {
			return err
		}
		if err := nativeWrite(source, styles); err != nil {
			return err
		}
	} else if tokyo {
		styles, err := nativeAcquireTokyoNightTmux(env)
		if err != nil {
			return err
		}
		if err := nativeWrite(source, styles); err != nil {
			return err
		}
	}
	// Custom sources are explicitly trusted tmux code; official sources include
	// only style commands. No shell or live server is run; the next config load
	// includes the theme after existing configuration.
	quoted := "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`").Replace(source) + "\""
	return nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return strings.HasSuffix(s, " # themeset") }, "source-file "+quoted+" # themeset"))
}

// Tmux's full author script changes status layout, clock and plugin settings.
// Select only its literal color-style commands, retaining existing user layout.
var filesTokyoNightTmuxStyle = regexp.MustCompile(`^(set|setw) -g ([a-z-]+) ("[a-zA-Z0-9,#=]+"|NONE,[a-zA-Z0-9,#=]+)$`)

func filesTokyoNightTmuxColors(data []byte) ([]byte, error) {
	var result strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		matches := filesTokyoNightTmuxStyle.FindStringSubmatch(line)
		if len(matches) == 0 {
			continue
		}
		switch matches[2] {
		case "mode-style", "message-style", "message-command-style", "pane-border-style", "pane-active-border-style", "status-style", "window-status-activity-style", "window-status-style":
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	if !strings.Contains(result.String(), "pane-active-border-style") || !strings.Contains(result.String(), "status-style") {
		return nil, errors.New("Tokyo Night tmux script has no native color styles")
	}
	return []byte(result.String()), nil
}

func nativeAcquireTokyoNightTmux(env model.Environment) ([]byte, error) {
	data, err := filesAcquireAsset(env, filesTokyoNightBase+filesTokyoNightAssets["tmux"])
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night tmux: %w", err)
	}
	data, err = filesTokyoNightTmuxColors(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var filesMoonflyTmuxStyle = regexp.MustCompile(`^(set|setw) -g ([a-z-]+) ("[a-zA-Z0-9,#=]+"|none)$`)

func filesMoonflyTmuxColors(data []byte) ([]byte, error) {
	var result strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		match := filesMoonflyTmuxStyle.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		switch match[2] {
		case "message-style", "pane-active-border-style", "pane-border-style", "status-style", "window-status-activity-style", "window-status-current-style":
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	if !strings.Contains(result.String(), "pane-active-border-style") || !strings.Contains(result.String(), "status-style") {
		return nil, errors.New("Moonfly tmux source has no native color styles")
	}
	return []byte(result.String()), nil
}

func nativeAcquireMoonflyTmux(env model.Environment) ([]byte, error) {
	data, err := filesAcquireAsset(env, filesMoonflySource+"moonfly.tmux")
	if err != nil {
		return nil, fmt.Errorf("acquire Moonfly tmux: %w", err)
	}
	return filesMoonflyTmuxColors(data)
}
