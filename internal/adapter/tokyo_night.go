package adapter

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// These files are generated and published by the Tokyo Night theme author.
// The night variant is selected consistently with the existing native ports.
const (
	filesTokyoNightBase       = "https://raw.githubusercontent.com/folke/tokyonight.nvim/cdc07ac78467a233fd62c493de29a17e0cf2b2b6/"
	filesTokyoNightLicense    = filesTokyoNightBase + "LICENSE"
	filesTokyoNightLicenseSum = "c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4"
)

type filesTokyoNightAsset struct {
	path, sha256 string
}

var filesTokyoNightAssets = map[string]filesTokyoNightAsset{
	"btop":    {"extras/btop/tokyonight_night.theme", "d1b0738fbd7afb81ce63b559f11fd77083b537c0309a60a01d79348f7b305230"},
	"eza":     {"extras/eza/tokyonight_night.yml", "cda1cc9f6bf5a477e1590c1542b3a8ea64e7eea52b4a98b7284ce429d57600e5"},
	"fzf":     {"extras/fzf/tokyonight_night.sh", "96793a02dbc2f6d4761af082179093f22002beb9d9e701aa4d8fab0c26029458"},
	"lazygit": {"extras/lazygit/tokyonight_night.yml", "866f23bb55e5adb07c36976c4d81108c750956a9e7dfa755451626973f222865"},
	"yazi":    {"extras/yazi/tokyonight_night.toml", "21a64a7ed55230359d180b5128ab92aa75e5aba9ec4820301e346db2fd04a4a1"},
	"syntax":  {"extras/sublime/tokyonight_night.tmTheme", "955c14a16b04917428ffa8b567e2d3760f872f1044a1ad157857001274dceecd"},
	"tmux":    {"extras/tmux/tokyonight_night.tmux", "49bee7903bb435e59c8d9d4bf74b6f7d4caa471c3fe1e67ca38634ca00347b84"},
}

func filesResolveTokyoNightPort(id, name string, _ model.Environment) (model.Port, bool, error) {
	asset, ok := filesTokyoNightAssets[id]
	if name != "tokyo-night" || !ok || id == "syntax" {
		return model.Port{}, false, nil
	}
	selected := name
	if id == "btop" {
		selected = "tokyonight_night"
	}
	return model.Port{Kind: "official", Name: selected, Source: filesTokyoNightBase + asset.path}, true, nil
}

func filesResolveTokyoNightYazi(name string, env model.Environment) (model.Port, bool, error) {
	port, found, err := filesResolveTokyoNightPort("yazi", name, env)
	if !found || err != nil {
		return port, found, err
	}
	installed, err := filesYaziFlavorInstalled(name, env)
	if err != nil || !installed {
		return port, err == nil, err
	}
	port.Kind = "native"
	port.File = filepath.Join(filesYaziFlavorRoot(name, env), "flavor.toml")
	port.Source = filesYaziSource
	return port, true, nil
}

// Only a registered source may enter acquisition; resolution is always read-only.
func filesAcquireTokyoNightPort(id string, port model.Port, env model.Environment) ([]byte, error) {
	asset, ok := filesTokyoNightAssets[id]
	if !ok || id == "syntax" || id == "tmux" || port.Kind != "official" || port.File != "" || port.Source != filesTokyoNightBase+asset.path || id == "btop" && port.Name != "tokyonight_night" || id != "btop" && port.Name != "tokyo-night" {
		return nil, fmt.Errorf("unregistered Tokyo Night %s port", id)
	}
	data, err := filesAcquireAsset(env, port.Source, asset.sha256)
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night %s: %w", id, err)
	}
	switch id {
	case "btop":
		err = filesBtopTheme(data)
	case "eza":
		err = filesValidateEzaFamily(data)
	case "fzf":
		data, err = filesTokyoNightFZF(data)
	case "lazygit":
		var config map[string]any
		config, err = filesYAML(data)
		if err == nil {
			gui, _ := config["gui"].(map[string]any)
			theme, _ := gui["theme"].(map[string]any)
			if len(theme) == 0 {
				err = errors.New("Tokyo Night Lazygit has no gui.theme")
			}
		}
	case "yazi":
		var config map[string]any
		config, err = filesTOML(data)
		if err == nil {
			mgr, _ := config["mgr"].(map[string]any)
			if len(mgr) == 0 {
				err = errors.New("Tokyo Night Yazi flavor has no [mgr] styles")
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("invalid Tokyo Night %s port: %w", id, err)
	}
	if _, err := filesAcquireAsset(env, filesTokyoNightLicense, filesTokyoNightLicenseSum); err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night license: %w", err)
	}
	return data, nil
}

// The author ships a shell script setting layout, preview and ANSI options.
// Extract only its literal --color entries; executing or importing shell code
// would overwrite the user's unrelated fzf configuration.
func filesTokyoNightFZF(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var colors strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), `\`))
		if !strings.HasPrefix(line, "--color=") {
			continue
		}
		colors.WriteString(line)
		colors.WriteByte('\n')
	}
	result := []byte(colors.String())
	if err := filesFZFOptions(result); err != nil {
		return nil, err
	}
	return result, nil
}

func filesApplyTokyoNightYazi(port model.Port, env model.Environment) error {
	if port.Name != "tokyo-night" {
		return errors.New("unregistered Tokyo Night Yazi flavor")
	}
	if port.Kind == "native" {
		if port.File != filepath.Join(filesYaziFlavorRoot(port.Name, env), "flavor.toml") || port.Source != filesYaziSource {
			return errors.New("unregistered native Tokyo Night Yazi flavor")
		}
		installed, err := filesYaziFlavorInstalled(port.Name, env)
		if err != nil {
			return err
		}
		if !installed {
			return errors.New("Tokyo Night Yazi flavor is not installed completely")
		}
		return filesSelectTokyoNightYazi(env)
	}
	if port.File != "" || port.Kind != "official" || port.Source != filesTokyoNightBase+filesTokyoNightAssets["yazi"].path {
		return errors.New("unregistered Tokyo Night Yazi flavor")
	}
	flavorData, err := filesAcquireTokyoNightPort("yazi", port, env)
	if err != nil {
		return err
	}
	syntax := filesTokyoNightAssets["syntax"]
	syntaxData, err := filesAcquireAsset(env, filesTokyoNightBase+syntax.path, syntax.sha256)
	if err != nil {
		return fmt.Errorf("acquire Tokyo Night Yazi syntax: %w", err)
	}
	return filesInstallTokyoNightYazi(env, flavorData, syntaxData)
}

func filesInstallTokyoNightYazi(env model.Environment, flavorData, syntaxData []byte) error {
	var theme struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(syntaxData, &theme); err != nil || theme.XMLName.Local != "plist" {
		return fmt.Errorf("invalid Tokyo Night Yazi syntax theme: %v", err)
	}
	root := filesYaziFlavorRoot("tokyo-night", env)
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("refusing to replace non-directory Yazi flavor %s", root)
		}
		for _, item := range []struct {
			name string
			data []byte
		}{{"flavor.toml", flavorData}, {"tmtheme.xml", syntaxData}} {
			existing, err := filesRead(filepath.Join(root, item.name), true)
			if err != nil {
				return err
			}
			if existing != nil && !bytes.Equal(existing, item.data) {
				return fmt.Errorf("refusing to replace installed Tokyo Night Yazi flavor %s", filepath.Join(root, item.name))
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		flavor, err := filesTable(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesAtomic(filepath.Join(root, "tmtheme.xml"), syntaxData); err != nil {
			return err
		}
		if err := filesAtomic(filepath.Join(root, "flavor.toml"), flavorData); err != nil {
			return err
		}
		flavor["dark"] = "tokyo-night"
		flavor["light"] = "tokyo-night"
		delete(flavor, "use")
		return nil
	})
}

func filesSelectTokyoNightYazi(env model.Environment) error {
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		flavor, err := filesTable(config, "flavor")
		if err != nil {
			return err
		}
		flavor["dark"] = "tokyo-night"
		flavor["light"] = "tokyo-night"
		delete(flavor, "use")
		return nil
	})
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
	asset := filesTokyoNightAssets["tmux"]
	data, err := filesAcquireAsset(env, filesTokyoNightBase+asset.path, asset.sha256)
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night tmux: %w", err)
	}
	data, err = filesTokyoNightTmuxColors(data)
	if err != nil {
		return nil, err
	}
	if _, err := filesAcquireAsset(env, filesTokyoNightLicense, filesTokyoNightLicenseSum); err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night license: %w", err)
	}
	return data, nil
}
