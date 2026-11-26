package adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/model"
)

const filesYaziSource = "https://yazi-rs.github.io/docs/flavors/overview/"

func filesResolveYazi(name string, env model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return filesResolveYaziDracula(env)
	}
	if port, found, err := filesResolveTokyoNightYazi(name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveRosePineYazi(name, env); found || err != nil {
		return port, found, err
	}
	if port, ok := filesOfficialYazi(name); ok {
		return port, true, nil
	}
	if filesYaziFamilyName(name) {
		return filesResolveYaziFamily(name, env)
	}
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	root := filepath.Join(filepath.Dir(filesPath("yazi", env)), "flavors", name+".yazi")
	data, err := filesRead(filepath.Join(root, "flavor.toml"), true)
	if err != nil {
		return model.Port{}, false, err
	}
	if len(data) == 0 {
		return model.Port{}, false, nil
	}
	if _, err := filesTOML(data); err != nil {
		return model.Port{}, false, err
	}
	if _, err := filesRead(filepath.Join(root, "tmtheme.xml"), false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Port{}, false, nil
		}
		return model.Port{}, false, err
	}
	return model.Port{Kind: "native", Name: name, File: filepath.Join(root, "flavor.toml"), Source: filesYaziSource}, true, nil
}

func filesApplyYazi(port model.Port, env model.Environment) error {
	if port.Name == "tokyo-night" && (port.Kind == "official" && port.File == "" || port.Kind == "native") {
		return filesApplyTokyoNightYazi(port, env)
	}
	if port.Name == "rose-pine" && (port.Kind == "official" && port.File == "" || port.Kind == "native") {
		return filesApplyRosePineYazi(port, env)
	}
	if port.Kind == "native" && filesYaziFamilyName(port.Name) {
		return filesApplyYaziFamily(port, env)
	}
	if port.Name == "dracula" && (port.Kind == "native" || port.Kind == "official" && port.File == "") {
		return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
			flavor, err := filesTable(config, "flavor")
			if err != nil {
				return err
			}
			if err := filesRemovePreviousYaziStyles(config, env); err != nil {
				return err
			}
			if err := filesInstallYaziDracula(env); err != nil {
				return err
			}
			flavor["dark"] = "dracula"
			flavor["light"] = "dracula"
			delete(flavor, "use")
			return nil
		})
	}
	var incoming map[string]any
	if port.Kind == "native" {
		if _, ok, err := filesResolveYazi(port.Name, env); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Yazi flavor %q is not installed", port.Name)
		}
		incoming = map[string]any{"flavor": map[string]any{"dark": port.Name, "light": port.Name}}
	} else {
		data, err := filesPortData("yazi", port, env)
		if err != nil {
			return err
		}
		incoming, err = filesTOML(data)
		if err != nil {
			return err
		}
		if len(incoming) == 0 {
			return errors.New("Yazi theme file is empty")
		}
	}
	// theme.toml is dedicated to styles. Recursively merge supplied sections,
	// retaining unspecified style overrides; yazi.toml/keymap.toml are untouched.
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		if port.Kind == "official" && port.File == "" {
			return filesApplyOfficialYazi(config, incoming, env)
		}
		filesMerge(config, incoming)
		return nil
	})
}
