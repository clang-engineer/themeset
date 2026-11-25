package adapter

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/clang-engineer/themeset/internal/model"
)

// Ghostty's macOS scripting API invokes the same reload_config action as its
// keyboard shortcut. Avoid SIGUSR2: Ghostty 1.4's hard signal reload may
// desynchronize attached tmux clients' terminal modes. A scratch --home must
// never reload the user's actual Ghostty instance.
func nativeReloadGhostty(env model.Environment) error {
	if env.OS != "darwin" || runtime.GOOS != "darwin" {
		return nil
	}
	account, err := user.Current()
	if err != nil {
		return fmt.Errorf("identify user for Ghostty reload: %w", err)
	}
	if filepath.Clean(env.Home) != filepath.Clean(account.HomeDir) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// `tell application` would start Ghostty when it is absent. Never launch a
	// terminal as a side effect of writing a theme for its next invocation.
	if err := exec.CommandContext(ctx, "pgrep", "-x", "-u", account.Uid, "ghostty").Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("find running Ghostty: %w", err)
	}
	const action = `tell application "Ghostty" to perform action "reload_config" on focused terminal of selected tab of front window`
	out, err := exec.CommandContext(ctx, "osascript", "-e", action).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload running Ghostty via AppleScript (theme file was saved): %w: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("Ghostty did not acknowledge reload_config (theme file was saved): %s", out)
	}
	return nil
}
