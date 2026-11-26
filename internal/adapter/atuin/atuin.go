package atuin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "atuin", Binary: "atuin", Remote: true, Resolve: filesResolveAtuin, Apply: filesApplyAtuin}
}

func filesAtuinThemes(env model.Environment) string {
	candidates := []string{
		filepath.Join(env.Home, ".atuin", "themes"),
		filepath.Join(filepath.Dir(shared.Path("atuin", env)), "themes"),
		filepath.Join(shared.ConfigHome(env), "atuin", "themes"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			return path
		}
	}
	return candidates[1]
}

func filesAtuinBuiltin(name string) bool {
	candidates := catalog.Port(name, "atuin").Native
	return len(candidates) == 1 && candidates[0] == name
}

func filesAtuinTheme(data []byte) (string, error) {
	config, err := shared.TOML(data)
	if err != nil {
		return "", err
	}
	theme, _ := config["theme"].(map[string]any)
	name, _ := theme["name"].(string)
	colors, ok := config["colors"].(map[string]any)
	if !shared.Name(name) || !ok || len(colors) == 0 {
		return "", errors.New("Atuin theme file requires [theme].name and a nonempty [colors] table")
	}
	for key, value := range colors {
		if text, ok := value.(string); !ok || text == "" {
			return "", fmt.Errorf("Atuin color %s must be a nonempty string", key)
		}
	}
	return name, nil
}

func filesResolveAtuin(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	for _, name := range spec.Native {
		if filesAtuinBuiltin(name) {
			return model.Port{Kind: "native", Name: name, Source: shared.AtuinThemesSource}, true, nil
		}
		if !shared.Name(name) {
			continue
		}
		path := filepath.Join(filesAtuinThemes(env), name+".toml")
		data, err := shared.Read(path, true)
		if err != nil {
			return model.Port{}, false, err
		}
		if len(data) == 0 {
			continue
		}
		actual, err := filesAtuinTheme(data)
		if err != nil {
			return model.Port{}, false, err
		}
		if actual != name {
			return model.Port{}, false, fmt.Errorf("Atuin theme %s declares name %q, expected %q", path, actual, name)
		}
		return model.Port{Kind: "native", Name: name, File: path, Source: shared.AtuinThemesSource}, true, nil
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyAtuin(port model.Port, env model.Environment) error {
	var asset []byte
	if port.Kind != "native" {
		var err error
		asset, err = filesPortData(port, env)
		if err != nil {
			return err
		}
		if port.Kind == "official" && port.File == "" && port.URL == "" {
			if err := filesValidateAtuinPort(port, asset); err != nil {
				return err
			}
		}
		actual, err := filesAtuinTheme(asset)
		if err != nil {
			return err
		}
		if port.URL != "" && port.Name != actual {
			return fmt.Errorf("remote Atuin theme declares name %q, expected %q", actual, port.Name)
		}
		port.Name = actual
		if filesAtuinBuiltin(port.Name) {
			return errors.New("custom Atuin theme must not shadow a built-in theme name")
		}
	} else if _, ok, err := filesResolveAtuin(model.ThemePort{Native: []string{port.Name}}, env); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("Atuin theme %q is not available", port.Name)
	}
	return shared.EditTOML(shared.Path("atuin", env), func(config map[string]any) error {
		theme, err := shared.Table(config, "theme")
		if err != nil {
			return err
		}
		if asset != nil {
			if port.URL != "" {
				if err := filesWriteRemoteAtuin(port, env, asset); err != nil {
					return err
				}
			} else if err := shared.Atomic(filepath.Join(filesAtuinThemes(env), port.Name+".toml"), asset); err != nil {
				return err
			}
		}
		theme["name"] = port.Name
		return nil
	})
}

func filesValidateAtuinPort(port model.Port, data []byte) error {
	name, err := filesAtuinTheme(data)
	if err != nil {
		return err
	}
	expected := port.Variant
	if expected == "" {
		expected = port.Name
	}
	if !shared.Name(expected) || name != expected {
		return fmt.Errorf("unexpected Atuin theme name %q", name)
	}
	return nil
}

func filesPortData(port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" || port.URL != "" {
		return shared.Custom(port, env)
	}
	if port.Kind == "official" {
		if !catalog.Verified("atuin", port) {
			return nil, fmt.Errorf("unregistered Atuin official port %q", port.Name)
		}
		data, err := shared.AcquireAsset(env, port.Source)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, fmt.Errorf("atuin requires an available official port or a registered native theme file")
}
