package shared

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/model"
)

// Path resolves a tool's theme configuration destination. File-format editing
// lives in files.go; platform and environment-specific locations live here.
func Path(id string, env model.Environment) string {
	if path := env.Paths[id]; path != "" {
		return Expand(path, env)
	}
	root := ConfigHome(env)
	switch id {
	case "starship":
		return filepath.Join(root, "starship.toml")
	case "television":
		return filepath.Join(root, "television", "config.toml")
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
