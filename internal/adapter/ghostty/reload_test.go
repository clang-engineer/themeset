package ghostty

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestGhosttyReloadUsesNativeActionOnlyForRealHome(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Ghostty AppleScript reload is macOS-only")
	}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	bin, marker := t.TempDir(), filepath.Join(t.TempDir(), "action")
	if err := os.WriteFile(filepath.Join(bin, "pgrep"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "osascript"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > '"+marker+"'\nprintf 'true\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := nativeReloadGhostty(model.Environment{Home: t.TempDir(), OS: "linux"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("scratch home reached live Ghostty action: %v", err)
	}
	if err := nativeReloadGhostty(model.Environment{Home: account.HomeDir, OS: "darwin"}); err != nil {
		t.Fatal(err)
	}
	invocation, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(invocation), "perform action \"reload_config\"") {
		t.Fatalf("native Ghostty action not invoked: %q (%v)", invocation, err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pgrep"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := nativeReloadGhostty(model.Environment{Home: account.HomeDir, OS: "darwin"}); err != nil {
		t.Fatalf("absent Ghostty should leave next launch to load the theme: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("absent Ghostty triggered automation: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bin, "pgrep"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "osascript"), []byte("#!/bin/sh\nprintf 'false\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := nativeReloadGhostty(model.Environment{Home: account.HomeDir, OS: "darwin"}); err == nil {
		t.Fatal("unacknowledged GUI reload reported success")
	}
}
