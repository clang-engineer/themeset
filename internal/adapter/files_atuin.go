package adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/model"
)

const filesAtuinSource = "https://docs.atuin.sh/latest/guide/theming/"

func filesAtuinThemes(env model.Environment) string {
	candidates := []string{
		filepath.Join(env.Home, ".atuin", "themes"),
		filepath.Join(filepath.Dir(filesPath("atuin", env)), "themes"),
		filepath.Join(filesConfigHome(env), "atuin", "themes"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			return path
		}
	}
	return candidates[1]
}

func filesAtuinBuiltin(name string) bool {
	switch name {
	case "default", "autumn", "marine", "pumpkin", "default-powershell", "(none)":
		return true
	}
	return false
}

func filesAtuinTheme(data []byte) (string, error) {
	config, err := filesTOML(data)
	if err != nil {
		return "", err
	}
	theme, _ := config["theme"].(map[string]any)
	name, _ := theme["name"].(string)
	colors, ok := config["colors"].(map[string]any)
	if !filesName(name) || !ok || len(colors) == 0 {
		return "", errors.New("Atuin theme file requires [theme].name and a nonempty [colors] table")
	}
	for key, value := range colors {
		if text, ok := value.(string); !ok || text == "" {
			return "", fmt.Errorf("Atuin color %s must be a nonempty string", key)
		}
	}
	return name, nil
}

func filesResolveAtuin(name string, env model.Environment) (model.Port, bool, error) {
	if port, ok := filesOfficialAtuin(name); ok {
		return port, true, nil
	}
	if port, found, err := filesResolveRosePinePort("atuin", name, env); found || err != nil {
		return port, found, err
	}
	if filesAtuinBuiltin(name) {
		return model.Port{Kind: "native", Name: name, Source: filesAtuinSource}, true, nil
	}
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	path := filepath.Join(filesAtuinThemes(env), name+".toml")
	data, err := filesRead(path, true)
	if err != nil {
		return model.Port{}, false, err
	}
	if len(data) == 0 {
		return model.Port{}, false, nil
	}
	actual, err := filesAtuinTheme(data)
	if err != nil {
		return model.Port{}, false, err
	}
	if actual != name {
		return model.Port{}, false, fmt.Errorf("Atuin theme %s declares name %q, expected %q", path, actual, name)
	}
	return model.Port{Kind: "native", Name: name, File: path, Source: filesAtuinSource}, true, nil
}

func filesApplyAtuin(port model.Port, env model.Environment) error {
	var asset []byte
	if port.Kind != "native" {
		var err error
		asset, err = filesPortData("atuin", port, env)
		if err != nil {
			return err
		}
		port.Name, err = filesAtuinTheme(asset)
		if err != nil {
			return err
		}
		if filesAtuinBuiltin(port.Name) {
			return errors.New("custom Atuin theme must not shadow a built-in theme name")
		}
	} else if _, ok, err := filesResolveAtuin(port.Name, env); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("Atuin theme %q is not available", port.Name)
	}
	return filesEditTOML(filesPath("atuin", env), func(config map[string]any) error {
		theme, err := filesTable(config, "theme")
		if err != nil {
			return err
		}
		if asset != nil {
			if err := filesAtomic(filepath.Join(filesAtuinThemes(env), port.Name+".toml"), asset); err != nil {
				return err
			}
		}
		theme["name"] = port.Name
		return nil
	})
}
