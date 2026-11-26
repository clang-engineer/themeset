package adapter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

const filesBtopSource = "https://github.com/aristocratos/btop#themes"

func filesBtopDirectories(env model.Environment) []string {
	directories := []string{filepath.Join(filepath.Dir(filesPath("btop", env)), "themes")}
	system := make([]string, 0, 3)
	if binary, err := exec.LookPath("btop"); err == nil {
		if resolved, err := filepath.EvalSymlinks(binary); err == nil {
			binary = resolved
		}
		system = append(system, filepath.Join(filepath.Dir(binary), "..", "share", "btop", "themes"))
	}
	system = append(system, "/usr/local/share/btop/themes", "/usr/share/btop/themes")
	// btop itself uses only the first nonempty system theme directory.
	for _, path := range system {
		entries, err := os.ReadDir(path)
		if err == nil && len(entries) > 0 {
			directories = append(directories, path)
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			directories = append(directories, path)
			break
		}
	}
	return directories
}

var filesBtopThemeLine = regexp.MustCompile(`^\s*theme\[[a-zA-Z0-9_]+\]\s*=\s*"[^"\r\n]*"\s*(#.*)?$`)
var filesBtopSelector = regexp.MustCompile(`^[\t ]*color_theme[\t ]*=`)

func filesBtopTheme(data []byte) error {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !filesBtopThemeLine.MatchString(line) {
			return errors.New("btop custom file must contain native theme[NAME]=\"COLOR\" entries only")
		}
		count++
	}
	if count == 0 {
		return errors.New("btop theme contains no theme entries")
	}
	return nil
}

func filesResolveBtop(name string, env model.Environment) (model.Port, bool, error) {
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	names := []string{name}
	switch name {
	case "gruvbox":
		names = []string{"gruvbox_dark", "gruvbox"}
	case "catppuccin-mocha":
		names = []string{"catppuccin_mocha", "catppuccin-mocha"}
	case "tokyo-night":
		names = []string{"tokyonight_night", "tokyo-night"}
	}
	for _, dir := range filesBtopDirectories(env) {
		for _, candidate := range names {
			path := filepath.Join(dir, candidate+".theme")
			data, err := filesRead(path, true)
			if err != nil {
				return model.Port{}, false, err
			}
			if len(data) == 0 {
				continue
			}
			if err := filesBtopTheme(data); err != nil {
				return model.Port{}, false, fmt.Errorf("parse %s: %w", path, err)
			}
			return model.Port{Kind: "native", Name: candidate, File: path, Source: filesBtopSource}, true, nil
		}
	}
	if port, found, err := filesResolveTokyoNightPort("btop", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveRosePinePort("btop", name, env); found || err != nil {
		return port, found, err
	}
	if name == "catppuccin-mocha" {
		return filesResolveBtopCatppuccin()
	}
	return filesResolveBtopUpstream(name)
}

func filesApplyBtop(port model.Port, env model.Environment) error {
	var asset []byte
	switch port.Kind {
	case "official":
		var err error
		if port.Source == filesBtopCatppuccinSource && port.Name == "catppuccin_mocha" {
			asset, err = filesAcquireBtopCatppuccin(env)
		} else if port.Source == filesTokyoNightBase+filesTokyoNightAssets["btop"].path && port.Name == "tokyonight_night" {
			asset, err = filesAcquireTokyoNightPort("btop", port, env)
		} else if port.Name == "rose-pine" {
			asset, err = filesAcquireRosePinePort("btop", port, env)
		} else {
			asset, err = filesAcquireBtopUpstream(port, env)
		}
		if err != nil {
			return err
		}
	case "native":
		if _, ok, err := filesResolveBtop(port.Name, env); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("btop theme %q is not installed", port.Name)
		}
	default:
		var err error
		asset, err = filesCustom(port, env)
		if err != nil {
			return err
		}
		if err := filesBtopTheme(asset); err != nil {
			return err
		}
		port.Name = "themeset"
	}
	path := filesPath("btop", env)
	data, err := filesRead(path, true)
	if err != nil {
		return err
	}
	setting := fmt.Sprintf("color_theme = %q", port.Name)
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if filesBtopSelector.MatchString(line) {
			lines[i] = setting
			found = true
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, setting, "")
	}
	if asset != nil {
		name := "themeset.theme"
		if port.Kind == "official" {
			name = port.Name + ".theme"
		}
		if err := filesAtomic(filepath.Join(filepath.Dir(path), "themes", name), asset); err != nil {
			return err
		}
	}
	return filesAtomic(path, []byte(strings.Join(lines, "\n")))
}
