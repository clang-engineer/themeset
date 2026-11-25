package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesFZFTestBinary(t *testing.T, env model.Environment, version string) {
	t.Helper()
	bin := filepath.Join(env.Home, "bin")
	path := filepath.Join(bin, "fzf")
	filesTestWrite(t, path, "#!/bin/sh\n[ \"$#\" -eq 1 ] && [ \"$1\" = --version ] || exit 9\nprintf '%s\\n' '"+version+"'\n")
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestFilesFZFNewOfficialPortsPreserveShellOptions(t *testing.T) {
	for _, name := range []string{"catppuccin-mocha", "gruvbox", "nord"} {
		t.Run(name, func(t *testing.T) {
			env := filesTestEnv(t)
			filesFZFTestBinary(t, env, "0.52.0 (test)")
			shell := filepath.Join(env.Home, ".zshrc")
			ordinary := filepath.Join(env.Home, "ordinary-fzf.opts")
			shellOptions := "export FZF_DEFAULT_OPTS='--height=40% --reverse --bind=ctrl-j:down'\n"
			fileOptions := "--layout=reverse\n--border=rounded\n"
			filesTestWrite(t, shell, shellOptions)
			filesTestWrite(t, ordinary, fileOptions)
			t.Setenv("FZF_DEFAULT_OPTS", "--height=40% --reverse")
			t.Setenv("FZF_DEFAULT_OPTS_FILE", ordinary)
			path := filesPath("fzf", env)
			filesTestWrite(t, path, "--color=fg:#010203\n")
			port, ok, err := filesResolveFZF(name, env)
			if err != nil || !ok || port.Kind != "official" || port.Name != name || !strings.Contains(port.Source, "/blob/") {
				t.Fatalf("official port unavailable: %#v, ok=%v, err=%v", port, ok, err)
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			options := string(filesTestRead(t, path))
			if err := filesFZFOptions([]byte(options)); err != nil {
				t.Fatal(err)
			}
			keys := make(map[string]bool)
			for _, line := range strings.Fields(options) {
				for _, color := range strings.Split(strings.TrimPrefix(line, "--color="), ",") {
					key, _, ok := strings.Cut(color, ":")
					if !ok || keys[key] {
						t.Fatalf("invalid or duplicate color option %q", color)
					}
					// The sole newer keys belong to the version-gated theme-author port.
					switch key {
					case "fg", "bg", "hl", "fg+", "bg+", "hl+", "info", "prompt", "pointer", "marker", "spinner", "header", "border", "gutter":
					case "selected-bg", "label":
						if name != "catppuccin-mocha" {
							t.Fatalf("modern color key %q escaped version-gated port", key)
						}
					default:
						t.Fatalf("non-stable or non-color option %q", color)
					}
					keys[key] = true
				}
			}
			for _, key := range []string{"fg", "bg", "hl", "fg+", "bg+", "hl+", "prompt", "marker", "border"} {
				if !keys[key] {
					t.Errorf("required fzf color role %q missing", key)
				}
			}
			if strings.Contains(options, "#010203") {
				t.Fatal("old theme options retained")
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			if string(filesTestRead(t, path)) != options || string(filesTestRead(t, shell)) != shellOptions || string(filesTestRead(t, ordinary)) != fileOptions {
				t.Fatal("reapply changed theme output or ordinary shell options")
			}
			filesTestWrite(t, path, "--height=50%\n")
			if err := filesApplyFZF(port, env); err == nil {
				t.Fatal("non-theme target options overwritten")
			}
			if string(filesTestRead(t, path)) != "--height=50%\n" {
				t.Fatal("non-theme target changed after rejection")
			}
		})
	}
}

func TestFilesFZFSelectedColorVersionBoundary(t *testing.T) {
	for _, tt := range []struct {
		version string
		want    bool
	}{
		{"0.51.9 (older)", false},
		{"0.52.0 (boundary)", true},
		{"0.75.0", true},
		{"1.0.0 (future)", true},
		{"0.9.99", false},
		{"0.520", false},
		{"0.52.0garbage", false},
		{"unknown", false},
		{"", false},
	} {
		t.Run(tt.version, func(t *testing.T) {
			if got := filesFZFSelectedColors([]byte(tt.version)); got != tt.want {
				t.Fatalf("version %q: got %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestFilesFZFIncompatibleVersionRejectsBeforeWriting(t *testing.T) {
	for _, version := range []string{"0.51.9", "unknown"} {
		t.Run(version, func(t *testing.T) {
			env := filesTestEnv(t)
			filesFZFTestBinary(t, env, version)
			if _, ok, err := filesResolveFZF("catppuccin-mocha", env); ok || err == nil || !strings.Contains(err.Error(), "0.52.0") {
				t.Fatalf("incompatible version resolved: ok=%v, err=%v", ok, err)
			}
			path := filesPath("fzf", env)
			original := "--color=fg:#010203\n"
			filesTestWrite(t, path, original)
			if err := filesApplyFZF(model.Port{Kind: "official", Name: "catppuccin-mocha"}, env); err == nil {
				t.Fatal("incompatible version applied")
			}
			if string(filesTestRead(t, path)) != original {
				t.Fatal("theme target changed before version rejection")
			}
			// A custom options file does not inherit the bundled asset's requirement.
			custom := filepath.Join(env.Home, "custom.opts")
			filesTestWrite(t, custom, "--color=fg:#123456\n")
			if err := filesApplyFZF(model.Port{Kind: "custom", Name: "catppuccin-mocha", File: custom}, env); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"gruvbox", "nord"} {
				port, ok, err := filesResolveFZF(name, env)
				if !ok || err != nil {
					t.Fatalf("stable-key preset %s requires modern fzf: %v", name, err)
				}
				if err := filesApplyFZF(port, env); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
