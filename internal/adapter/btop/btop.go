package btop

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "btop", Binary: "btop", Remote: true, Resolve: filesResolveBtop, Apply: filesApplyBtop}
}

func filesBtopDirectories(env model.Environment) []string {
	directories := []string{filepath.Join(filepath.Dir(shared.Path("btop", env)), "themes")}
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

func filesResolveBtop(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	for _, candidate := range spec.Native {
		if !shared.Name(candidate) {
			continue
		}
		for _, dir := range filesBtopDirectories(env) {
			path := filepath.Join(dir, candidate+".theme")
			data, err := shared.Read(path, true)
			if err != nil {
				return model.Port{}, false, err
			}
			if len(data) == 0 {
				continue
			}
			if err := filesBtopTheme(data); err != nil {
				return model.Port{}, false, fmt.Errorf("parse %s: %w", path, err)
			}
			return model.Port{Kind: "native", Name: candidate, File: path, Source: shared.BtopThemesSource}, true, nil
		}
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyBtop(port model.Port, env model.Environment) error {
	var asset []byte
	switch port.Kind {
	case "official":
		if !catalog.Verified("btop", port) {
			return fmt.Errorf("unregistered btop official theme %q", port.Name)
		}
		var err error
		switch port.Format {
		case "btop-cat":
			asset, err = filesAcquireBtopCatppuccin(port, env)
		case "btop-tokyo":
			asset, err = filesAcquireTokyoNightPort(port, env)
		case "btop-rose":
			asset, err = filesAcquireRosePinePort(port, env)
		case "btop-upstream":
			asset, err = filesAcquireBtopUpstream(port, env)
		default:
			return fmt.Errorf("unsupported btop official format %q", port.Format)
		}
		if err != nil {
			return err
		}
	case "native":
		if _, ok, err := filesResolveBtop(model.ThemePort{Native: []string{port.Name}}, env); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("btop theme %q is not installed", port.Name)
		}
	default:
		var err error
		asset, err = shared.Custom(port, env)
		if err != nil {
			return err
		}
		if err := filesBtopTheme(asset); err != nil {
			return err
		}
		port.Name = "themeset"
	}
	path := shared.Path("btop", env)
	data, err := shared.Read(path, true)
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
		if err := shared.Atomic(filepath.Join(filepath.Dir(path), "themes", name), asset); err != nil {
			return err
		}
	}
	return shared.Atomic(path, []byte(strings.Join(lines, "\n")))
}

func filesAcquireBtopCatppuccin(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire btop Catppuccin Mocha: %w", err)
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid btop Catppuccin Mocha: %w", err)
	}
	if port.LicenseURL != "" {
		if _, err := shared.AcquireAsset(env, port.LicenseURL); err != nil {
			return nil, fmt.Errorf("acquire btop Catppuccin Mocha license: %w", err)
		}
	}
	return data, nil
}

func filesAcquireRosePinePort(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireRosePinePort("btop", port, env)
	if err != nil {
		return nil, err
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid Rosé Pine btop theme: %w", err)
	}
	return data, nil
}

func filesAcquireTokyoNightPort(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireTokyoNightPort("btop", port, env)
	if err != nil {
		return nil, err
	}
	if err := filesBtopTheme(data); err != nil {
		return nil, fmt.Errorf("invalid Tokyo Night btop port: %w", err)
	}
	return data, nil
}
