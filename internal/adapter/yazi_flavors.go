package adapter

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesResolveRosePineYazi(name string, env model.Environment) (model.Port, bool, error) {
	if name != "rose-pine" {
		return model.Port{}, false, nil
	}
	port := model.Port{Kind: "official", Name: name, Source: rosePineYaziSource}
	installed, err := filesYaziFlavorInstalled(name, env)
	if err != nil || !installed {
		return port, err == nil, err
	}
	root := filesYaziFlavorRoot(name, env)
	owned, err := filesYaziMatchesSource(root, rosePineYaziSource)
	if err != nil {
		return model.Port{}, false, err
	}
	if !owned {
		port.Kind = "native"
		port.File = filepath.Join(root, "flavor.toml")
	}
	return port, true, nil
}

func filesInstallRosePineYazi(env model.Environment) error {
	root := filesYaziFlavorRoot("rose-pine", env)
	content := make(map[string][]byte, len(rosePineYaziAssets))
	for _, path := range rosePineYaziAssets {
		data, err := filesAcquireAsset(env, rosePineYaziBase+path)
		if err != nil {
			return fmt.Errorf("acquire Rosé Pine Yazi %s: %w", path, err)
		}
		content[filepath.Base(path)] = data
	}
	config, err := filesTOML(content["flavor.toml"])
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return errors.New("Rosé Pine Yazi flavor has no [mgr] styles")
	}
	var syntax struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(content["tmtheme.xml"], &syntax); err != nil || syntax.XMLName.Local != "plist" {
		return fmt.Errorf("invalid Rosé Pine Yazi syntax: %v", err)
	}
	return filesInstallOfficialYaziFlavor(root, rosePineYaziSource, content)
}

func filesApplyRosePineYazi(port model.Port, env model.Environment) error {
	if port.Name != "rose-pine" || port.Kind != "official" && port.Kind != "native" || port.Kind == "official" && port.Source != rosePineYaziSource || port.Kind == "native" && port.File != filepath.Join(filesYaziFlavorRoot("rose-pine", env), "flavor.toml") {
		return errors.New("unregistered Rosé Pine Yazi flavor")
	}
	if port.Kind == "native" {
		installed, err := filesYaziFlavorInstalled("rose-pine", env)
		if err != nil || !installed {
			return fmt.Errorf("Rosé Pine native Yazi flavor unavailable: %v", err)
		}
	}
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if port.Kind == "official" {
			if err := filesInstallRosePineYazi(env); err != nil {
				return err
			}
		}
		flavor, err := filesTable(config, "flavor")
		if err != nil {
			return err
		}
		flavor["dark"] = "rose-pine"
		flavor["light"] = "rose-pine"
		delete(flavor, "use")
		return nil
	})
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
	root := filesYaziFlavorRoot(name, env)
	owned, err := filesYaziMatchesSource(root, port.Source)
	if err != nil {
		return model.Port{}, false, err
	}
	if !owned {
		port.Kind = "native"
		port.File = filepath.Join(root, "flavor.toml")
		port.Source = filesYaziSource
	}
	return port, true, nil
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
	if port.File != "" || port.Kind != "official" || port.Source != filesTokyoNightBase+filesTokyoNightAssets["yazi"] {
		return errors.New("unregistered Tokyo Night Yazi flavor")
	}
	flavorData, err := filesAcquireTokyoNightPort("yazi", port, env)
	if err != nil {
		return err
	}
	syntax := filesTokyoNightAssets["syntax"]
	syntaxData, err := filesAcquireAsset(env, filesTokyoNightBase+syntax)
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
	config, err := filesTOML(flavorData)
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return errors.New("Tokyo Night Yazi flavor has no [mgr] styles")
	}
	root := filesYaziFlavorRoot("tokyo-night", env)
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		flavor, err := filesTable(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesInstallOfficialYaziFlavor(root, filesTokyoNightBase+filesTokyoNightAssets["yazi"], map[string][]byte{
			"flavor.toml": flavorData,
			"tmtheme.xml": syntaxData,
		}); err != nil {
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

// Official installs carry a provenance marker; independently installed complete
// flavors do not need one to be usable as native flavors.
const filesYaziSourceMarker = ".themeset-source"

func filesYaziMatchesSource(root, source string) (bool, error) {
	data, err := filesRead(filepath.Join(root, filesYaziSourceMarker), true)
	return string(data) == source+"\n", err
}

func filesYaziDraculaRoot(env model.Environment) string {
	return filesYaziFlavorRoot("dracula", env)
}

// Independently installed compatible flavors remain native, even when their
// bytes happen to match a previously downloaded official release.
func filesYaziDraculaInstalled(env model.Environment) (bool, error) {
	return filesYaziFlavorInstalled("dracula", env)
}

func filesResolveYaziDracula(env model.Environment) (model.Port, bool, error) {
	port := model.Port{Kind: "official", Name: "dracula", Source: filesYaziDraculaSource}
	installed, err := filesYaziDraculaInstalled(env)
	if err != nil {
		return model.Port{}, false, err
	}
	if !installed {
		return port, true, nil
	}
	matches, err := filesYaziMatchesSource(filesYaziDraculaRoot(env), filesYaziDraculaSource)
	if err != nil {
		return model.Port{}, false, err
	}
	if !matches {
		port.Kind = "native"
		port.File = filepath.Join(filesYaziDraculaRoot(env), "flavor.toml")
	}
	return port, true, nil
}

func filesInstallYaziDracula(env model.Environment) error {
	installed, err := filesYaziDraculaInstalled(env)
	if err != nil || installed {
		return err
	}
	// Acquire and validate everything before making a flavor visible. Existing
	// partial files must match the current upstream bytes, not a historical hash.
	root := filesYaziDraculaRoot(env)
	info, err := os.Lstat(root)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if exists && !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory Dracula flavor %s", root)
	}
	owned := false
	if exists {
		marker, err := filesRead(filepath.Join(root, filesYaziSourceMarker), true)
		if err != nil {
			return err
		}
		if marker != nil && string(marker) != filesYaziDraculaSource+"\n" {
			return fmt.Errorf("incomplete Dracula flavor has conflicting %s; refusing to overwrite it", filesYaziSourceMarker)
		}
		owned = marker != nil
	}
	assets := make([][]byte, len(filesYaziDraculaAssets))
	for i, asset := range filesYaziDraculaAssets {
		assets[i], err = filesAcquireAsset(env, filesYaziDraculaRaw+asset.name)
		if err != nil {
			return fmt.Errorf("acquire Yazi Dracula %s: %w", asset.name, err)
		}
		if len(assets[i]) == 0 {
			return fmt.Errorf("Yazi Dracula %s is empty", asset.name)
		}
		if !exists {
			continue
		}
		previous, err := filesRead(filepath.Join(root, asset.name), true)
		if err != nil {
			return err
		}
		if !owned && previous != nil && !bytes.Equal(previous, assets[i]) {
			return fmt.Errorf("incomplete Dracula flavor has conflicting %s; refusing to overwrite it", asset.name)
		}
	}
	config, err := filesTOML(assets[0])
	if err != nil {
		return fmt.Errorf("validate downloaded Dracula flavor: %w", err)
	}
	mgr, ok := config["mgr"].(map[string]any)
	if !ok || len(mgr) == 0 {
		return errors.New("downloaded Dracula flavor requires a nonempty [mgr] table")
	}
	var theme struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(assets[1], &theme); err != nil {
		return fmt.Errorf("validate downloaded Dracula syntax theme: %w", err)
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".themeset-dracula-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if exists {
		// Preserve extra files in an incomplete preexisting flavor. CopyFS
		// deliberately refuses symlinks rather than following or replacing them.
		if err := os.CopyFS(stage, os.DirFS(root)); err != nil {
			return fmt.Errorf("stage existing Dracula flavor: %w", err)
		}
	}
	for i, asset := range filesYaziDraculaAssets {
		if err := filesAtomic(filepath.Join(stage, asset.name), assets[i]); err != nil {
			return err
		}
	}
	if err := filesAtomic(filepath.Join(stage, filesYaziSourceMarker), []byte(filesYaziDraculaSource+"\n")); err != nil {
		return err
	}
	if !exists {
		return os.Rename(stage, root)
	}
	backup, err := os.MkdirTemp(parent, ".themeset-dracula-previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(root, backup); err != nil {
		return err
	}
	if err := os.Rename(stage, root); err != nil {
		if restoreErr := os.Rename(backup, root); restoreErr != nil {
			return fmt.Errorf("install Dracula flavor: %w; restore prior flavor from %s: %v", err, backup, restoreErr)
		}
		return err
	}
	return os.RemoveAll(backup)
}

func filesYaziManagedStylesPath(env model.Environment) string {
	return filepath.Join(filepath.Dir(filesPath("yazi", env)), ".themeset-catppuccin-mocha-styles.toml")
}

// A managed-style snapshot records the bytes actually installed, so a later
// switch removes only still-unchanged imported style leaves. Older installs
// have no snapshot; compare against the current upstream source conservatively.
func filesRemovePreviousYaziStyles(config map[string]any, env model.Environment) error {
	mgr, ok := config["mgr"].(map[string]any)
	if !ok || mgr["syntect_theme"] != filepath.Join(filepath.Dir(filesPath("yazi", env)), "themeset-catppuccin-mocha.tmTheme") {
		return nil
	}
	data, err := filesRead(filesYaziManagedStylesPath(env), true)
	if err != nil {
		return err
	}
	if data == nil {
		port, registered := filesOfficialYazi("catppuccin-mocha")
		if !registered {
			return errors.New("Catppuccin Yazi source is not registered")
		}
		data, err = filesPortData("yazi", port, env)
		if err != nil {
			return err
		}
	}
	previous, err := filesTOML(data)
	if err != nil {
		return fmt.Errorf("parse previously applied Yazi styles: %w", err)
	}
	if err := filesValidateYaziPort(data); err != nil {
		return fmt.Errorf("invalid previously applied Yazi styles: %w", err)
	}
	delete(mgr, "syntect_theme")
	for _, section := range []string{"app", "mgr", "tabs", "mode", "indicator", "status", "input", "pick", "confirm", "spot", "completion", "tasks", "which", "help", "notify"} {
		styles, ok := previous[section].(map[string]any)
		if !ok {
			continue
		}
		target, ok := config[section].(map[string]any)
		if !ok {
			continue
		}
		for key, value := range styles {
			style, ok := value.(map[string]any)
			if !ok || !filesYaziStyle(style) {
				continue
			}
			existing, ok := target[key].(map[string]any)
			if !ok {
				continue
			}
			for leaf, old := range style {
				if reflect.DeepEqual(existing[leaf], old) {
					delete(existing, leaf)
				}
			}
			if len(existing) == 0 {
				delete(target, key)
			}
		}
		if len(target) == 0 {
			delete(config, section)
		}
	}
	return nil
}
