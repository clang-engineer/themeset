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

// Two-file official flavors share one installation and activation path.
// Source is the style URL unless the catalog provides a separate asset URL.
func filesApplyTwoFileYazi(port model.Port, env model.Environment) error {
	if port.Kind != "official" || port.Format != "yazi-flavor-pair" || port.File != "" || port.URL != "" || !shared.Name(port.Name) || port.Source == "" || port.SyntaxURL == "" {
		return errors.New("unregistered two-file Yazi flavor")
	}
	flavorURL := port.AssetURL
	if flavorURL == "" {
		flavorURL = port.Source
	}
	flavorData, err := shared.AcquireAsset(env, flavorURL)
	if err != nil {
		return fmt.Errorf("acquire %s Yazi flavor: %w", port.Name, err)
	}
	syntaxData, err := shared.AcquireAsset(env, port.SyntaxURL)
	if err != nil {
		return fmt.Errorf("acquire %s Yazi syntax: %w", port.Name, err)
	}
	return filesInstallTwoFileYazi(port, env, flavorData, syntaxData)
}

func filesInstallTwoFileYazi(port model.Port, env model.Environment, flavorData, syntaxData []byte) error {
	if port.Kind != "official" || port.Format != "yazi-flavor-pair" || port.File != "" || !shared.Name(port.Name) || port.Source == "" {
		return errors.New("unregistered two-file Yazi flavor")
	}
	config, err := shared.TOML(flavorData)
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return fmt.Errorf("%s Yazi flavor has no [mgr] styles", port.Name)
	}
	var syntax struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(syntaxData, &syntax); err != nil || syntax.XMLName.Local != "plist" {
		return fmt.Errorf("invalid %s Yazi syntax: %v", port.Name, err)
	}
	return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesInstallOfficialYaziFlavor(filesYaziFlavorRoot(port.Name, env), port.Source, map[string][]byte{
			"flavor.toml": flavorData,
			"tmtheme.xml": syntaxData,
		}); err != nil {
			return err
		}
		flavor["dark"] = port.Name
		flavor["light"] = port.Name
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

func filesInstallYaziBundle(port model.Port, env model.Environment) error {
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
		return fmt.Errorf("refusing to replace non-directory %s flavor %s", port.Name, root)
	}
	owned := false
	if exists {
		marker, err := shared.Read(filepath.Join(root, filesYaziSourceMarker), true)
		if err != nil {
			return err
		}
		if marker != nil && string(marker) != port.Source+"\n" {
			return fmt.Errorf("incomplete %s flavor has conflicting %s; refusing to overwrite it", port.Name, filesYaziSourceMarker)
		}
		owned = marker != nil
	}
	assets := make([][]byte, 4)
	names := [...]string{"flavor.toml", "tmtheme.xml", "LICENSE", "LICENSE-tmtheme"}
	urls := [...]string{port.AssetURL, port.SyntaxURL, port.LicenseURL, port.LicenseSyntaxURL}
	for i, asset := range names {
		assets[i], err = shared.AcquireAsset(env, urls[i])
		if err != nil {
			return fmt.Errorf("acquire Yazi %s %s: %w", port.Name, asset, err)
		}
		if len(assets[i]) == 0 {
			return fmt.Errorf("Yazi %s %s is empty", port.Name, asset)
		}
		if !exists {
			continue
		}
		previous, err := shared.Read(filepath.Join(root, asset), true)
		if err != nil {
			return err
		}
		if !owned && previous != nil && !bytes.Equal(previous, assets[i]) {
			return fmt.Errorf("incomplete %s flavor has conflicting %s; refusing to overwrite it", port.Name, asset)
		}
	}
	config, err := shared.TOML(assets[0])
	if err != nil {
		return fmt.Errorf("validate downloaded %s flavor: %w", port.Name, err)
	}
	mgr, ok := config["mgr"].(map[string]any)
	if !ok || len(mgr) == 0 {
		return errors.New("downloaded Yazi flavor requires a nonempty [mgr] table")
	}
	var theme struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(assets[1], &theme); err != nil {
		return fmt.Errorf("validate downloaded %s syntax theme: %w", port.Name, err)
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".themeset-flavor-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if exists {
		// Preserve extra files in an incomplete preexisting flavor. CopyFS
		// deliberately refuses symlinks rather than following or replacing them.
		if err := os.CopyFS(stage, os.DirFS(root)); err != nil {
			return fmt.Errorf("stage existing %s flavor: %w", port.Name, err)
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
	backup, err := os.MkdirTemp(parent, ".themeset-flavor-previous-*")
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
			return fmt.Errorf("install %s flavor: %w; restore prior flavor from %s: %v", port.Name, err, backup, restoreErr)
		}
		return err
	}
	return os.RemoveAll(backup)
}

func filesYaziManagedStylesPath(name string, env model.Environment) string {
	return filepath.Join(filepath.Dir(shared.Path("yazi", env)), ".themeset-"+name+"-styles.toml")
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
	if filepath.Dir(syntaxPath) != directory {
		return nil
	}
	filename := filepath.Base(syntaxPath)
	if !strings.HasPrefix(filename, "themeset-") || !strings.HasSuffix(filename, ".tmTheme") {
		return nil
	}
	name := strings.TrimSuffix(strings.TrimPrefix(filename, "themeset-"), ".tmTheme")
	official := !strings.HasPrefix(name, "custom-")
	if !shared.Name(name) {
		return nil
	}
	catppuccin := official && name == "catppuccin-mocha"
	marker := filesYaziManagedStylesPath(name, env)
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
	if official {
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
