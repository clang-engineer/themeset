package adapter

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/clang-engineer/themeset/internal/model"
)

const filesYaziDraculaRevision = "38c08d8eb9ea0d28c8c911a249a08f27ba2151ac"
const filesYaziDraculaSource = "https://github.com/yazi-rs/flavors/tree/" + filesYaziDraculaRevision + "/dracula.yazi"
const filesYaziDraculaRaw = "https://raw.githubusercontent.com/yazi-rs/flavors/" + filesYaziDraculaRevision + "/dracula.yazi/"

// The complete runtime flavor and both upstream licenses, not the repository's
// preview image or README. Each digest describes the exact immutable raw bytes.
var filesYaziDraculaAssets = []struct {
	name, digest string
}{
	{"flavor.toml", "b57d21e1a30a971f81c2541f400e8bf60c3b05d79682bba34b3072389258550a"},
	{"tmtheme.xml", "6767fe7cf5a2759d108207156f500c189a3dec216bd6f4f60b9b4bc09fbf8a5a"},
	{"LICENSE", "06a2b04a7ed4f030a87d10b884fc1a2215c5e91b371f69dfe173448e834f3752"},
	{"LICENSE-tmtheme", "21ab5b51be129196203d9e9cd1a2d51451fe913bc972fc1462a2fe11633692a1"},
}

func filesYaziDraculaRoot(env model.Environment) string {
	return filesYaziFlavorRoot("dracula", env)
}

// Independently installed compatible flavors remain native: never replace a
// complete user flavor just because it differs from our registered revision.
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
	for _, asset := range filesYaziDraculaAssets {
		data, err := filesRead(filepath.Join(filesYaziDraculaRoot(env), asset.name), true)
		if err != nil {
			return model.Port{}, false, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != asset.digest {
			port.Kind = "native"
			port.File = filepath.Join(filesYaziDraculaRoot(env), "flavor.toml")
			break
		}
	}
	return port, true, nil
}

func filesInstallYaziDracula(env model.Environment) error {
	installed, err := filesYaziDraculaInstalled(env)
	if err != nil || installed {
		return err
	}
	// Acquire everything before making the destination directory visible. A
	// failed download may populate the verified cache, but never an active flavor.
	root := filesYaziDraculaRoot(env)
	info, err := os.Lstat(root)
	exists := err == nil
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if exists && !info.IsDir() {
		return fmt.Errorf("refusing to replace non-directory Dracula flavor %s", root)
	}
	assets := make([][]byte, len(filesYaziDraculaAssets))
	for i, asset := range filesYaziDraculaAssets {
		data, err := filesRead(filepath.Join(root, asset.name), true)
		if err != nil {
			return err
		}
		if data != nil {
			digest := sha256.Sum256(data)
			if hex.EncodeToString(digest[:]) != asset.digest {
				return fmt.Errorf("incomplete Dracula flavor has conflicting %s; refusing to overwrite it", asset.name)
			}
			assets[i] = data
		}
	}
	for i, asset := range filesYaziDraculaAssets {
		if assets[i] != nil {
			continue
		}
		assets[i], err = filesAcquireAsset(env, filesYaziDraculaRaw+asset.name, asset.digest)
		if err != nil {
			return fmt.Errorf("acquire Yazi Dracula %s: %w", asset.name, err)
		}
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

// The old bundled Catppuccin adapter imported only Style tables and wrote this
// exact syntect path. That marker establishes ownership, not a color match alone.
// Remove only imported leaves that still have the bundled value; edited leaves
// and unrelated layout, icon, filename and user syntax overrides remain intact.
func filesRemovePreviousYaziStyles(config map[string]any, env model.Environment) error {
	mgr, ok := config["mgr"].(map[string]any)
	if !ok || mgr["syntect_theme"] != filepath.Join(filepath.Dir(filesPath("yazi", env)), "themeset-catppuccin-mocha.tmTheme") {
		return nil
	}
	previous, err := filesTOML(filesYaziCatppuccin)
	if err != nil {
		return err
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
