package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func environment(home string) (model.Environment, error) {
	var err error
	configHome := ""
	configHomeExplicit := false
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return model.Environment{}, fmt.Errorf("find home directory: %w", err)
		}
		configHome, configHomeExplicit = os.LookupEnv("XDG_CONFIG_HOME")
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return model.Environment{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	} else if !filepath.IsAbs(configHome) {
		return model.Environment{}, errors.New("XDG_CONFIG_HOME must be an absolute path")
	}
	return model.Environment{Home: home, ConfigHome: configHome, ConfigHomeExplicit: configHomeExplicit, OS: runtime.GOOS}, nil
}

func localPath(path, home, base string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		path = home
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	} else if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported home abbreviation %q; use ~/", path)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	return filepath.Clean(path), nil
}

func loadConfig(path string, env model.Environment) (model.Config, string, error) {
	if path == "" {
		path = filepath.Join(env.ConfigHome, "themeset", "config.json")
	} else {
		var err error
		cwd, err := os.Getwd()
		if err != nil {
			return model.Config{}, "", err
		}
		path, err = localPath(path, env.Home, cwd)
		if err != nil {
			return model.Config{}, "", err
		}
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		// A missing default config is optional; an explicit path is checked by Run.
		return model.Config{}, path, err
	}
	if err != nil {
		return model.Config{}, path, fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()
	var config *model.Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return model.Config{}, path, fmt.Errorf("decode config %s: %w", path, err)
	}
	if config == nil {
		return model.Config{}, path, fmt.Errorf("config %s must be a JSON object", path)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return model.Config{}, path, fmt.Errorf("decode config %s: %w", path, err)
	}
	base := filepath.Dir(path)
	for id, value := range config.Paths {
		resolved, err := localPath(value, env.Home, base)
		if err != nil {
			return model.Config{}, path, fmt.Errorf("paths.%s: %w", id, err)
		}
		config.Paths[id] = resolved
	}
	for theme, definition := range config.Themes {
		for id, port := range definition.Custom {
			if port.Format != "" || port.Variant != "" || port.LicenseURL != "" || port.AssetURL != "" || port.LicenseSyntaxURL != "" {
				return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s: format, variant and bundled asset URLs are reserved for bundled declarations", theme, id)
			}
			if port.URL != "" {
				if port.File != "" {
					return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s: file and url are mutually exclusive", theme, id)
				}
				if err := shared.ValidateAssetSource(port.URL); err != nil {
					return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s.url: %w", theme, id, err)
				}
			}
			if port.URL != "" && port.Name == "" {
				port.Name = theme
			}
			if id == "yazi" && port.URL != "" && port.SyntaxURL == "" {
				return model.Config{}, path, fmt.Errorf("themes.%s.custom.yazi.syntax_url is required for remote styles", theme)
			}
			if port.SyntaxURL != "" {
				if port.URL == "" || id != "yazi" {
					return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s.syntax_url requires a Yazi url", theme, id)
				}
				if err := shared.ValidateAssetSource(port.SyntaxURL); err != nil {
					return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s.syntax_url: %w", theme, id, err)
				}
			}
			port.Kind = "custom"
			port.File, err = localPath(port.File, env.Home, base)
			if err != nil {
				return model.Config{}, path, fmt.Errorf("themes.%s.custom.%s.file: %w", theme, id, err)
			}
			definition.Custom[id] = port
		}
	}
	return *config, path, nil
}
