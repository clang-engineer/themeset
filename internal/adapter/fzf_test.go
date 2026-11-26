package adapter

import (
	"bytes"
	"io"
	"net/http"
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

const filesFZFTestPresets = `var PRESETS = {
  'Gruvbox Dark': {
    fg:'#ebdbb2', bg:'#282828', hl:'#fabd2f', 'current-fg':'#ebdbb2', 'current-bg':'#3c3836', 'current-hl':'#fabd2f',
    info:'#83a598', prompt:'#bdae93', pointer:'#83a598', marker:'#a8c4b9', spinner:'#fabd2f', header:'#a89984',
    border:'#504945', gutter:'#3c3836',
  },
  'Nord': {
    fg:'#D8DEE9', bg:'#2E3440', hl:'#A3BE8C', 'current-fg':'#D8DEE9', 'current-bg':'#434C5E', 'current-hl':'#A3BE8C',
    info:'#88C0D0', prompt:'#81A1C1', pointer:'#BF616A', marker:'#d08888', spinner:'#EBCB8B', header:'#8FBCBB',
    border:'#4C566A', gutter:'#434C5E',
  },
};`

const filesFZFTestMoonfly = `# Moonfly fzf script
export FZF_DEFAULT_OPTS="$FZF_DEFAULT_OPTS \
  --color bg:#080808 \
  --color bg+:#262626 \
  --color border:#2e2e2e \
  --color fg:#b2b2b2 \
  --color fg+:#e4e4e4 \
  --color gutter:#262626 \
  --color header:#80a0ff \
  --color hl+:#f09479 \
  --color hl:#f09479 \
  --color info:#cfcfb0 \
  --color marker:#f09479 \
  --color pointer:#ff5189 \
  --color prompt:#80a0ff \
  --color spinner:#36c692
"`

func filesFZFTestSource(t *testing.T, sources map[string]*string) {
	t.Helper()
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		source, ok := sources[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected Fzf source %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(*source)), Header: make(http.Header)}, nil
	})
}

func TestFilesFZFOfficialExtractsCurrentSourceWithoutExecution(t *testing.T) {
	for _, tt := range []struct{ name, source, initial string }{
		{"dracula", filesFZFSource, "### Setup\n```sh\nexport FZF_DEFAULT_OPTS='--color=fg:#f8f8f2,bg:#282a36 --color=hl:#bd93f9'\n```\n"},
		{"moonfly", filesMoonflySource + "moonfly-fzf.sh", filesFZFTestMoonfly},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := filesTestEnv(t)
			source := tt.initial
			filesFZFTestSource(t, map[string]*string{tt.source: &source})
			port, ok, err := filesResolveFZF(tt.name, env)
			if !ok || err != nil || port.Source != tt.source {
				t.Fatalf("resolve: %#v, %v", port, err)
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			path := filesPath("fzf", env)
			before := filesTestRead(t, path)
			if tt.name == "dracula" {
				source = strings.ReplaceAll(source, "#f8f8f2", "#112233")
			} else {
				source = strings.ReplaceAll(source, "#080808", "#112233")
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(before, filesTestRead(t, path)) || !strings.Contains(string(filesTestRead(t, path)), "#112233") {
				t.Fatal("reapply did not fetch changed official colors")
			}
			before = filesTestRead(t, path)
			source += "\n--color=fg:#001122 --preview=touch-bad\n"
			if tt.name == "dracula" {
				source = strings.Replace(source, "--color=hl:#bd93f9", "--color=hl:#bd93f9 --preview=bad", 1)
			} else {
				source = strings.Replace(source, "--color bg:#112233", "--color bg:#112233 --preview=bad", 1)
			}
			if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, filesTestRead(t, path)) {
				t.Fatal("unsafe official shell source changed theme")
			}
		})
	}
}

func TestFilesFZFNewOfficialPortsPreserveShellOptions(t *testing.T) {
	for _, name := range []string{"catppuccin-mocha", "gruvbox", "nord"} {
		t.Run(name, func(t *testing.T) {
			env := filesTestEnv(t)
			filesFZFTestBinary(t, env, "0.52.0 (test)")
			catppuccin := "--color=bg+:#313244,bg:#1E1E2E,spinner:#F5E0DC,hl:#F38BA8\n--color=fg:#CDD6F4,header:#F38BA8,info:#CBA6F7,pointer:#F5E0DC\n--color=marker:#B4BEFE,fg+:#CDD6F4,prompt:#CBA6F7,hl+:#F38BA8\n--color=selected-bg:#45475A\n--color=border:#6C7086,label:#CDD6F4\n"
			presets := filesFZFTestPresets
			filesFZFTestSource(t, map[string]*string{
				filesFZFCatppuccinSource: &catppuccin,
				filesFZFPresetsSource:     &presets,
			})
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
			if err != nil || !ok || port.Kind != "official" || port.Name != name || !strings.HasPrefix(port.Source, "https://raw.githubusercontent.com/") {
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

func TestFilesFZFPresetsRejectInvalidOrMissingColorData(t *testing.T) {
	env := filesTestEnv(t)
	source := filesFZFTestPresets
	filesFZFTestSource(t, map[string]*string{filesFZFPresetsSource: &source})
	port, ok, err := filesResolveFZF("gruvbox", env)
	if !ok || err != nil {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	path := filesPath("fzf", env)
	filesTestWrite(t, path, "--color=fg:#123456\n")
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	before := filesTestRead(t, path)
	source = strings.Replace(filesFZFTestPresets, "#ebdbb2", "#aabbcc", 1)
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, filesTestRead(t, path)) || !strings.Contains(string(filesTestRead(t, path)), "#aabbcc") {
		t.Fatal("preset did not use current source")
	}
	before = filesTestRead(t, path)
	for _, malformed := range []string{
		strings.Replace(filesFZFTestPresets, "'Gruvbox Dark'", "'Wrong'", 1),
		strings.Replace(filesFZFTestPresets, "fg:'#ebdbb2'", "fg:'bad'", 1),
		strings.Replace(filesFZFTestPresets, "fg:'#ebdbb2'", "fg:'#ebdbb2', preview:'danger'", 1),
	} {
		source = malformed
		if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, filesTestRead(t, path)) {
			t.Fatalf("invalid upstream JS changed theme: %v", err)
		}
	}
}

func TestFilesFZFCatppuccinFetchesCurrentColorsAndRejectsUnsafeOptions(t *testing.T) {
	env := filesTestEnv(t)
	filesFZFTestBinary(t, env, "0.52.0")
	source := "--color=fg:#CDD6F4,bg:#1E1E2E,selected-bg:#45475A\n"
	filesFZFTestSource(t, map[string]*string{filesFZFCatppuccinSource: &source})
	port, ok, err := filesResolveFZF("catppuccin-mocha", env)
	if !ok || err != nil {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	path := filesPath("fzf", env)
	source = strings.Replace(source, "#CDD6F4", "#123456", 1)
	if err := filesApplyFZF(port, env); err != nil || !strings.Contains(string(filesTestRead(t, path)), "#123456") {
		t.Fatalf("current Catppuccin source not applied: %v", err)
	}
	before := filesTestRead(t, path)
	source = "--color=fg:#123456\n--preview=touch-dangerous\n"
	if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, filesTestRead(t, path)) {
		t.Fatal("unsafe Catppuccin source changed fzf configuration")
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
			presets := filesFZFTestPresets
			filesFZFTestSource(t, map[string]*string{filesFZFPresetsSource: &presets})
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
			// A custom options file does not inherit the official port's requirement.
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
