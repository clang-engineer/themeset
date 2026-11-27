package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/testutil"
)

func TestTmuxReloadTargetsCurrentServerAndRestoresBaseLayout(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	env := testutil.Env(t)
	socketDir, err := os.MkdirTemp("/tmp", "themeset-reload-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "socket")
	root := filepath.Join(env.Home, ".tmux.conf")
	managed := filepath.Join(env.ConfigHome, "themeset", "tmux.conf")
	testutil.Write(t, root, "set -g status-left 'base layout'\nsource-file \""+managed+"\"\n")
	testutil.Write(t, managed, "set -g status-left 'plugin layout'\n")
	command := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("tmux", append([]string{"-S", socket}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	command("-f", root, "new-session", "-d", "-s", "smoke")
	t.Cleanup(func() { exec.Command("tmux", "-S", socket, "kill-server").Run() })
	if got := command("show-option", "-gv", "status-left"); got != "plugin layout" {
		t.Fatalf("startup theme not active: %q", got)
	}
	testutil.Write(t, managed, "set -g status-style none\n")
	t.Setenv("TMUX", socket+",123,0")
	// --home from a scratch run must not reload an existing live server.
	if err := nativeReloadTmux(managed, root, env); err != nil {
		t.Fatal(err)
	}
	if got := command("show-option", "-gv", "status-left"); got != "plugin layout" {
		t.Fatalf("scratch apply reloaded live server: %q", got)
	}
	if err := reloadCurrentTmux(managed, root, env, os.Getenv("TMUX")); err != nil {
		t.Fatal(err)
	}
	if got := command("show-option", "-gv", "status-left"); got != "base layout" {
		t.Fatalf("base layout not restored on full reload: %q", got)
	}
	testutil.Write(t, root, "# source-file \""+managed+"\"\nset -g status-left 'unrelated root'\n")
	testutil.Write(t, managed, "set -g status-left 'managed only'\n")
	if err := reloadCurrentTmux(managed, root, env, os.Getenv("TMUX")); err != nil {
		t.Fatal(err)
	}
	if got := command("show-option", "-gv", "status-left"); got != "managed only" {
		t.Fatalf("unrelated top-level config reloaded: %q", got)
	}
	if err := reloadCurrentTmux(managed, root, env, "not-a-socket,123,0"); err == nil {
		t.Fatal("invalid tmux socket accepted")
	}
	if err := reloadCurrentTmux(managed, root, env, filepath.Join(t.TempDir(), "absent")+",123,0"); err == nil || !strings.Contains(err.Error(), "theme file was saved") {
		t.Fatalf("reload failure not reported after writing config: %v", err)
	}
}
