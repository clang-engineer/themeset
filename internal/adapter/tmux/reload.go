package tmux

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// Reload only the server that owns the current tmux client. A scratch --home
// must never source test settings into the user's running server.
func nativeReloadTmux(path, root string, env model.Environment) error {
	value := os.Getenv("TMUX")
	if value == "" {
		return nil
	}
	account, err := user.Current()
	if err != nil {
		return fmt.Errorf("identify user for tmux reload: %w (theme file was saved)", err)
	}
	if filepath.Clean(env.Home) != filepath.Clean(account.HomeDir) {
		return nil
	}
	return reloadCurrentTmux(path, root, env, value)
}

func reloadCurrentTmux(path, root string, env model.Environment, value string) error {
	socket, _, ok := strings.Cut(value, ",")
	if !ok || !filepath.IsAbs(socket) || strings.ContainsAny(socket, "\r\n\x00") {
		return fmt.Errorf("invalid current tmux socket (theme file was saved)")
	}
	// A top-level config that directly includes the managed file restores its
	// base layout first (e.g. OMT). Otherwise source only the changed file:
	// reloading an unrelated top-level config could replace the user's layout.
	if filepath.Clean(root) != filepath.Clean(path) && !tmuxRootIncludes(root, path) {
		root = path
	}
	if _, err := shared.Command(env, "tmux", "-S", socket, "source-file", root); err != nil {
		return fmt.Errorf("reload current tmux server from %s (theme file was saved): %w", root, err)
	}
	return nil
}

func tmuxRootIncludes(root, path string) bool {
	data, err := os.ReadFile(root)
	if err != nil {
		return false
	}
	for len(data) != 0 {
		line, rest, _ := bytes.Cut(data, []byte{'\n'})
		line = bytes.TrimSpace(line)
		if (bytes.HasPrefix(line, []byte("source-file ")) || bytes.HasPrefix(line, []byte("source "))) &&
			bytes.Contains(line, []byte(path)) {
			return true
		}
		data = rest
	}
	return false
}
