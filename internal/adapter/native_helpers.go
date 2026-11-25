package adapter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/clang-engineer/themeset/internal/model"
)

func nativeConfigHome(env model.Environment) string {
	if env.ConfigHome != "" {
		return nativeExpand(env.ConfigHome, env)
	}
	return filepath.Join(env.Home, ".config")
}

func nativeExpand(path string, env model.Environment) string {
	if path == "~" {
		return env.Home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(env.Home, path[2:])
	}
	if path != "" && !filepath.IsAbs(path) {
		return filepath.Join(env.Home, path)
	}
	return path
}

func nativePath(id string, env model.Environment, fallback string) string {
	if path := env.Paths[id]; path != "" {
		return nativeExpand(path, env)
	}
	return fallback
}

// Native inspection must not accidentally consult the invoking user's home.
func nativeCommand(env model.Environment, binary string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = env.Home
	cmd.Env = []string{}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "BAT_CONFIG_PATH", "BAT_THEME", "BAT_CONFIG_DIR", "BAT_CACHE_PATH", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT", "DELTA_FEATURES", "DELTA_SYNTAX_THEME", "HARLEQUIN_CONFIG_PATH", "NO_COLOR", "CLICOLOR_FORCE", "FORCE_COLOR":
			continue
		}
		if strings.HasPrefix(key, "GIT_CONFIG_KEY_") || strings.HasPrefix(key, "GIT_CONFIG_VALUE_") {
			continue
		}
		cmd.Env = append(cmd.Env, item)
	}
	cmd.Env = append(cmd.Env, "HOME="+env.Home, "XDG_CONFIG_HOME="+nativeConfigHome(env), "XDG_DATA_HOME="+filepath.Join(env.Home, ".local", "share"), "XDG_CACHE_HOME="+filepath.Join(env.Home, ".cache"), "NO_COLOR=1", "TERM=dumb", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+nativePath("delta", env, filepath.Join(env.Home, ".gitconfig")))
	if path := env.Paths["bat"]; path != "" {
		cmd.Env = append(cmd.Env, "BAT_CONFIG_PATH="+nativeExpand(path, env))
	}
	if path := env.Paths["harlequin"]; path != "" {
		cmd.Env = append(cmd.Env, "HARLEQUIN_CONFIG_PATH="+nativeExpand(path, env))
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", binary, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func nativeRead(path string) ([]byte, error) {
	return filesRead(path, true)
}

func nativeWrite(path string, data []byte) error {
	return filesAtomic(path, data)
}

func nativeReplaceLines(data []byte, match func(string) bool, line string) []byte {
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	result := make([]string, 0, len(lines)+1)
	for _, existing := range lines {
		if !match(existing) {
			result = append(result, existing)
		}
	}
	if len(result) == 1 && result[0] == "" {
		result = nil
	}
	result = append(result, line)
	return []byte(strings.Join(result, "\n") + "\n")
}

func nativeThemeKey(theme string) string {
	return strings.NewReplacer("_", "-", " ", "-").Replace(strings.ToLower(strings.TrimSpace(theme)))
}

var nativeANSI = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func nativeListContains(out []byte, name string) bool {
	clean := nativeANSI.ReplaceAllString(string(out), "")
	for _, line := range strings.Split(clean, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSuffix(line, " (light)")
		line = strings.TrimSuffix(line, " (dark)")
		line = strings.TrimSuffix(line, " (resources)")
		line = strings.TrimSuffix(line, " (user)")
		if variant, theme, ok := strings.Cut(line, "\t"); ok && (variant == "dark" || variant == "light") {
			line = theme
		}
		if line == name {
			return true
		}
	}
	return false
}

func nativeSelection(port model.Port, env model.Environment, parse func([]byte) (string, error)) (string, error) {
	if port.File != "" {
		data, err := os.ReadFile(nativeExpand(port.File, env))
		if err != nil {
			return "", err
		}
		return parse(data)
	}
	if strings.TrimSpace(port.Name) == "" || strings.ContainsAny(port.Name, "\r\n\x00") {
		return "", fmt.Errorf("a nonempty native theme name is required")
	}
	return port.Name, nil
}
