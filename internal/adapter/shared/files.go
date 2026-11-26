package shared

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

func Expand(path string, env model.Environment) string {
	if path == "~" {
		return env.Home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(env.Home, path[2:])
	}
	return path
}

func ConfigHome(env model.Environment) string {
	if env.ConfigHome != "" {
		return Expand(env.ConfigHome, env)
	}
	return filepath.Join(env.Home, ".config")
}

func Path(id string, env model.Environment) string {
	if path := env.Paths[id]; path != "" {
		return Expand(path, env)
	}
	root := ConfigHome(env)
	switch id {
	case "starship":
		return filepath.Join(root, "starship.toml")
	case "fzf":
		return filepath.Join(root, "themeset", "fzf.opts")
	case "yazi":
		if directory, ok := os.LookupEnv("YAZI_CONFIG_HOME"); ok {
			return filepath.Join(directory, "theme.toml")
		}
		return filepath.Join(root, id, "theme.toml")
	case "atuin":
		modern := filepath.Join(env.Home, ".atuin", "config.toml")
		legacy := filepath.Join(root, id, "config.toml")
		if _, err := os.Lstat(modern); !errors.Is(err, os.ErrNotExist) {
			return modern
		}
		if _, err := os.Lstat(legacy); !errors.Is(err, os.ErrNotExist) {
			return legacy
		}
		return modern
	case "lazygit", "lazydocker":
		if env.OS == "darwin" && !env.ConfigHomeExplicit {
			root = filepath.Join(env.Home, "Library", "Application Support")
			legacy := filepath.Join(root, "jesseduffield", id)
			if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
				return filepath.Join(legacy, "config.yml")
			}
		}
		return filepath.Join(root, id, "config.yml")
	case "btop":
		return filepath.Join(root, id, "btop.conf")
	case "eza":
		if directory, ok := os.LookupEnv("EZA_CONFIG_DIR"); ok {
			return filepath.Join(directory, "theme.yml")
		}
		if env.OS == "darwin" && !env.ConfigHomeExplicit {
			root = filepath.Join(env.Home, "Library", "Application Support")
		}
		return filepath.Join(root, id, "theme.yml")
	default:
		panic("unknown file adapter: " + id)
	}
}

// Read only regular files, following symlinks rather than replacing them. In
// particular, a dangling symlink is an error, not a missing configuration file.
func Read(path string, optional bool) ([]byte, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) && optional {
		if _, linkErr := os.Lstat(path); errors.Is(linkErr, os.ErrNotExist) {
			return nil, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

// The rename is on the destination filesystem. Existing modes and symlink
// objects are retained; new files are private until explicitly shared by users.
func Atomic(path string, data []byte) error {
	mode := os.FileMode(0600)
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			info, err = os.Stat(path)
			if err != nil {
				return err
			}
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		mode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".themeset-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func TOML(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func YAML(data []byte) (map[string]any, error) {
	result := make(map[string]any)
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&result); err != nil && err != io.EOF {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("configuration must contain one YAML document")
		}
		return nil, err
	}
	if result == nil {
		result = make(map[string]any)
	}
	return result, nil
}

func Table(config map[string]any, key string) (map[string]any, error) {
	if value, exists := config[key]; exists && value != nil {
		if table, ok := value.(map[string]any); ok {
			return table, nil
		}
		return nil, fmt.Errorf("%s must be a table/mapping", key)
	}
	table := make(map[string]any)
	config[key] = table
	return table, nil
}

func Merge(dst, src map[string]any) {
	for key, value := range src {
		incoming, incomingMap := value.(map[string]any)
		existing, existingMap := dst[key].(map[string]any)
		if incomingMap && existingMap {
			Merge(existing, incoming)
		} else {
			dst[key] = value
		}
	}
}

func EditTOML(path string, change func(map[string]any) error) error {
	data, err := Read(path, true)
	if err != nil {
		return err
	}
	config, err := TOML(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := change(config); err != nil {
		return err
	}
	data, err = toml.Marshal(config)
	if err != nil {
		return err
	}
	return Atomic(path, data)
}

func EditYAML(path string, change func(map[string]any) error) error {
	data, err := Read(path, true)
	if err != nil {
		return err
	}
	config, err := YAML(data)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if err := change(config); err != nil {
		return err
	}
	data, err = yaml.Marshal(config)
	if err != nil {
		return err
	}
	return Atomic(path, data)
}

func Custom(port model.Port, env model.Environment) ([]byte, error) {
	if port.File == "" {
		return nil, errors.New("custom port requires an app-native file")
	}
	return Read(Expand(port.File, env), false)
}

func Name(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
