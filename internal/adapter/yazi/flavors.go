package yazi

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func filesInstallRosePineYazi(port model.Port, env model.Environment) error {
	root := filesYaziFlavorRoot(port.Name, env)
	content := make(map[string][]byte, 2)
	for name, url := range map[string]string{"flavor.toml": port.AssetURL, "tmtheme.xml": port.SyntaxURL} {
		data, err := shared.AcquireAsset(env, url)
		if err != nil {
			return fmt.Errorf("acquire Rosé Pine Yazi %s: %w", name, err)
		}
		content[name] = data
	}
	config, err := shared.TOML(content["flavor.toml"])
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
	return filesInstallOfficialYaziFlavor(root, port.Source, content)
}

func filesApplyRosePineYazi(port model.Port, env model.Environment) error {
	if port.Kind != "official" || port.Format != "yazi-flavor-rose" || port.File != "" {
		return errors.New("unregistered Rosé Pine Yazi flavor")
	}
	return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesInstallRosePineYazi(port, env); err != nil {
			return err
		}
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		flavor["dark"] = "rose-pine"
		flavor["light"] = "rose-pine"
		delete(flavor, "use")
		return nil
	})
}
func filesApplyTokyoNightYazi(port model.Port, env model.Environment) error {
	if port.Kind != "official" || port.Format != "yazi-flavor-tokyo" {
		return errors.New("unregistered Tokyo Night Yazi flavor")
	}
	if port.File != "" || port.Kind != "official" || port.Source == "" {
		return errors.New("unregistered Tokyo Night Yazi flavor")
	}
	flavorData, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return err
	}
	if err := filesValidateYaziPort(flavorData); err != nil {
		return fmt.Errorf("invalid Tokyo Night yazi port: %w", err)
	}
	syntaxData, err := shared.AcquireAsset(env, port.SyntaxURL)
	if err != nil {
		return fmt.Errorf("acquire Tokyo Night Yazi syntax: %w", err)
	}
	return filesInstallTokyoNightYazi(env, port.Source, flavorData, syntaxData)
}

func filesInstallTokyoNightYazi(env model.Environment, source string, flavorData, syntaxData []byte) error {
	var theme struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(syntaxData, &theme); err != nil || theme.XMLName.Local != "plist" {
		return fmt.Errorf("invalid Tokyo Night Yazi syntax theme: %v", err)
	}
	config, err := shared.TOML(flavorData)
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return errors.New("Tokyo Night Yazi flavor has no [mgr] styles")
	}
	root := filesYaziFlavorRoot("tokyo-night", env)
	return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesInstallOfficialYaziFlavor(root, source, map[string][]byte{
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

// Official installs carry a provenance marker; independently installed complete
// flavors do not need one to be usable as native flavors.
const filesYaziSourceMarker = ".themeset-source"

func filesYaziMatchesSource(root, source string) (bool, error) {
	data, err := shared.Read(filepath.Join(root, filesYaziSourceMarker), true)
	return string(data) == source+"\n", err
}

func filesYaziDraculaRoot(env model.Environment) string {
	return filesYaziFlavorRoot("dracula", env)
}

func filesInstallYaziDracula(port model.Port, env model.Environment) error {
	installed, err := filesYaziFlavorInstalled(port.Name, env)
	if err != nil || installed {
		return err
	}
	// Acquire and validate everything before making a flavor visible. Existing
	// partial files must match the current upstream bytes, not a historical hash.
	root := filesYaziFlavorRoot(port.Name, env)
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
		marker, err := shared.Read(filepath.Join(root, filesYaziSourceMarker), true)
		if err != nil {
			return err
		}
		if marker != nil && string(marker) != port.Source+"\n" {
			return fmt.Errorf("incomplete Dracula flavor has conflicting %s; refusing to overwrite it", filesYaziSourceMarker)
		}
		owned = marker != nil
	}
	assets := make([][]byte, 4)
	names := [...]string{"flavor.toml", "tmtheme.xml", "LICENSE", "LICENSE-tmtheme"}
	urls := [...]string{port.AssetURL, port.SyntaxURL, port.LicenseURL, port.LicenseSyntaxURL}
	for i, asset := range names {
		assets[i], err = shared.AcquireAsset(env, urls[i])
		if err != nil {
			return fmt.Errorf("acquire Yazi Dracula %s: %w", asset, err)
		}
		if len(assets[i]) == 0 {
			return fmt.Errorf("Yazi Dracula %s is empty", asset)
		}
		if !exists {
			continue
		}
		previous, err := shared.Read(filepath.Join(root, asset), true)
		if err != nil {
			return err
		}
		if !owned && previous != nil && !bytes.Equal(previous, assets[i]) {
			return fmt.Errorf("incomplete Dracula flavor has conflicting %s; refusing to overwrite it", asset)
		}
	}
	config, err := shared.TOML(assets[0])
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
	for i, asset := range names {
		if err := shared.Atomic(filepath.Join(stage, asset), assets[i]); err != nil {
			return err
		}
	}
	if err := shared.Atomic(filepath.Join(stage, filesYaziSourceMarker), []byte(port.Source+"\n")); err != nil {
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
	return filepath.Join(filepath.Dir(shared.Path("yazi", env)), ".themeset-catppuccin-mocha-styles.toml")
}

// A managed-style snapshot records the bytes actually installed, so a later
// switch removes only still-unchanged imported style leaves. Older installs
// have no snapshot; compare against the current upstream source conservatively.
func filesRemovePreviousYaziStyles(config map[string]any, env model.Environment) error {
	mgr, ok := config["mgr"].(map[string]any)
	if !ok {
		return nil
	}
	syntaxPath, ok := mgr["syntect_theme"].(string)
	if !ok {
		return nil
	}
	directory := filepath.Dir(shared.Path("yazi", env))
	catppuccin := syntaxPath == filepath.Join(directory, "themeset-catppuccin-mocha.tmTheme")
	marker := filesYaziManagedStylesPath(env)
	if !catppuccin {
		if filepath.Dir(syntaxPath) != directory {
			return nil
		}
		filename := filepath.Base(syntaxPath)
		if !strings.HasPrefix(filename, "themeset-custom-") || !strings.HasSuffix(filename, ".tmTheme") {
			return nil
		}
		name := strings.TrimSuffix(strings.TrimPrefix(filename, "themeset-custom-"), ".tmTheme")
		if !shared.Name(name) {
			return nil
		}
		marker = filepath.Join(directory, ".themeset-custom-"+name+"-styles.toml")
	}
	data, err := shared.Read(marker, true)
	if err != nil {
		return err
	}
	if data == nil {
		if !catppuccin {
			return nil
		}
		data, err = shared.AcquireAsset(env, catalog.Port("catppuccin-mocha", "yazi").Official.Source)
		if err != nil {
			return err
		}
	}
	previous, err := shared.TOML(data)
	if err != nil {
		return fmt.Errorf("parse previously applied Yazi styles: %w", err)
	}
	if catppuccin {
		if err := filesValidateYaziPort(data); err != nil {
			return fmt.Errorf("invalid previously applied Yazi styles: %w", err)
		}
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
