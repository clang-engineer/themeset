package fzf

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func filesFZFTestBinary(t *testing.T, env model.Environment, version string) {
	t.Helper()
	bin := filepath.Join(env.Home, "bin")
	path := filepath.Join(bin, "fzf")
	testutil.Write(t, path, "#!/bin/sh\n[ \"$#\" -eq 1 ] && [ \"$1\" = --version ] || exit 9\nprintf '%s\\n' '"+version+"'\n")
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
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		source, ok := sources[r.URL.String()]
		if !ok {
			t.Fatalf("unexpected Fzf source %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(*source)), Header: make(http.Header)}, nil
	})
}

func TestFilesFZFOfficialExtractsCurrentSourceWithoutExecution(t *testing.T) {
	for _, tt := range []struct{ name, source, initial string }{
		{"dracula", shared.FZFDraculaSource, "### Setup\n```sh\nexport FZF_DEFAULT_OPTS='--color=fg:#f8f8f2,bg:#282a36 --color=hl:#bd93f9'\n```\n"},
		{"moonfly", shared.MoonflySource + "moonfly-fzf.sh", filesFZFTestMoonfly},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := testutil.Env(t)
			source := tt.initial
			filesFZFTestSource(t, map[string]*string{tt.source: &source})
			port, ok, err := filesResolveFZF(catalog.Port(tt.name, "fzf"), env)
			if !ok || err != nil || port.Source != tt.source {
				t.Fatalf("resolve: %#v, %v", port, err)
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			path := shared.Path("fzf", env)
			before := testutil.Read(t, path)
			if tt.name == "dracula" {
				source = strings.ReplaceAll(source, "#f8f8f2", "#112233")
			} else {
				source = strings.ReplaceAll(source, "#080808", "#112233")
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(before, testutil.Read(t, path)) || !strings.Contains(string(testutil.Read(t, path)), "#112233") {
				t.Fatal("reapply did not fetch changed official colors")
			}
			before = testutil.Read(t, path)
			source += "\n--color=fg:#001122 --preview=touch-bad\n"
			if tt.name == "dracula" {
				source = strings.Replace(source, "--color=hl:#bd93f9", "--color=hl:#bd93f9 --preview=bad", 1)
			} else {
				source = strings.Replace(source, "--color bg:#112233", "--color bg:#112233 --preview=bad", 1)
			}
			if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, testutil.Read(t, path)) {
				t.Fatal("unsafe official shell source changed theme")
			}
		})
	}
}

func TestFilesFZFNewOfficialPortsPreserveShellOptions(t *testing.T) {
	for _, name := range []string{"catppuccin-mocha", "gruvbox", "nord"} {
		t.Run(name, func(t *testing.T) {
			env := testutil.Env(t)
			filesFZFTestBinary(t, env, "0.52.0 (test)")
			catppuccin := "--color=bg+:#313244,bg:#1E1E2E,spinner:#F5E0DC,hl:#F38BA8\n--color=fg:#CDD6F4,header:#F38BA8,info:#CBA6F7,pointer:#F5E0DC\n--color=marker:#B4BEFE,fg+:#CDD6F4,prompt:#CBA6F7,hl+:#F38BA8\n--color=selected-bg:#45475A\n--color=border:#6C7086,label:#CDD6F4\n"
			presets := filesFZFTestPresets
			filesFZFTestSource(t, map[string]*string{
				shared.FZFCatppuccinSource: &catppuccin,
				shared.FZFPresetsSource:    &presets,
			})
			shell := filepath.Join(env.Home, ".zshrc")
			ordinary := filepath.Join(env.Home, "ordinary-fzf.opts")
			shellOptions := "export FZF_DEFAULT_OPTS='--height=40% --reverse --bind=ctrl-j:down'\n"
			fileOptions := "--layout=reverse\n--border=rounded\n"
			testutil.Write(t, shell, shellOptions)
			testutil.Write(t, ordinary, fileOptions)
			t.Setenv("FZF_DEFAULT_OPTS", "--height=40% --reverse")
			t.Setenv("FZF_DEFAULT_OPTS_FILE", ordinary)
			path := shared.Path("fzf", env)
			testutil.Write(t, path, "--color=fg:#010203\n")
			port, ok, err := filesResolveFZF(catalog.Port(name, "fzf"), env)
			if err != nil || !ok || port.Kind != "official" || port.Name != name || !strings.HasPrefix(port.Source, "https://raw.githubusercontent.com/") {
				t.Fatalf("official port unavailable: %#v, ok=%v, err=%v", port, ok, err)
			}
			if err := filesApplyFZF(port, env); err != nil {
				t.Fatal(err)
			}
			options := string(testutil.Read(t, path))
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
			if string(testutil.Read(t, path)) != options || string(testutil.Read(t, shell)) != shellOptions || string(testutil.Read(t, ordinary)) != fileOptions {
				t.Fatal("reapply changed theme output or ordinary shell options")
			}
			testutil.Write(t, path, "--height=50%\n")
			if err := filesApplyFZF(port, env); err == nil {
				t.Fatal("non-theme target options overwritten")
			}
			if string(testutil.Read(t, path)) != "--height=50%\n" {
				t.Fatal("non-theme target changed after rejection")
			}
		})
	}
}

func TestFilesFZFPresetsRejectInvalidOrMissingColorData(t *testing.T) {
	env := testutil.Env(t)
	source := filesFZFTestPresets
	filesFZFTestSource(t, map[string]*string{shared.FZFPresetsSource: &source})
	port, ok, err := filesResolveFZF(catalog.Port("gruvbox", "fzf"), env)
	if !ok || err != nil {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	path := shared.Path("fzf", env)
	testutil.Write(t, path, "--color=fg:#123456\n")
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	before := testutil.Read(t, path)
	source = strings.Replace(filesFZFTestPresets, "#ebdbb2", "#aabbcc", 1)
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, testutil.Read(t, path)) || !strings.Contains(string(testutil.Read(t, path)), "#aabbcc") {
		t.Fatal("preset did not use current source")
	}
	before = testutil.Read(t, path)
	for _, malformed := range []string{
		strings.Replace(filesFZFTestPresets, "'Gruvbox Dark'", "'Wrong'", 1),
		strings.Replace(filesFZFTestPresets, "fg:'#ebdbb2'", "fg:'bad'", 1),
		strings.Replace(filesFZFTestPresets, "fg:'#ebdbb2'", "fg:'#ebdbb2', preview:'danger'", 1),
	} {
		source = malformed
		if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, testutil.Read(t, path)) {
			t.Fatalf("invalid upstream JS changed theme: %v", err)
		}
	}
}

func TestFilesFZFCatppuccinFetchesCurrentColorsAndRejectsUnsafeOptions(t *testing.T) {
	env := testutil.Env(t)
	filesFZFTestBinary(t, env, "0.52.0")
	source := "--color=fg:#CDD6F4,bg:#1E1E2E,selected-bg:#45475A\n"
	filesFZFTestSource(t, map[string]*string{shared.FZFCatppuccinSource: &source})
	port, ok, err := filesResolveFZF(catalog.Port("catppuccin-mocha", "fzf"), env)
	if !ok || err != nil {
		t.Fatalf("resolve: %#v, %v", port, err)
	}
	if err := filesApplyFZF(port, env); err != nil {
		t.Fatal(err)
	}
	path := shared.Path("fzf", env)
	source = strings.Replace(source, "#CDD6F4", "#123456", 1)
	if err := filesApplyFZF(port, env); err != nil || !strings.Contains(string(testutil.Read(t, path)), "#123456") {
		t.Fatalf("current Catppuccin source not applied: %v", err)
	}
	before := testutil.Read(t, path)
	source = "--color=fg:#123456\n--preview=touch-dangerous\n"
	if err := filesApplyFZF(port, env); err == nil || !bytes.Equal(before, testutil.Read(t, path)) {
		t.Fatal("unsafe Catppuccin source changed fzf configuration")
	}
}

func TestFilesFZFRejectsForgedOfficialMetadataBeforeFetching(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("fzf", env)
	original := "--color=fg:#010203\n"
	testutil.Write(t, path, original)
	for _, tt := range []struct {
		name   string
		change func(*model.Port)
	}{
		{"source", func(p *model.Port) { p.Source = "https://example.test/unregistered.js" }},
		{"variant", func(p *model.Port) { p.Variant = "Nord" }},
		{"format", func(p *model.Port) { p.Format = "fzf-rc" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			port := *catalog.Port("gruvbox", "fzf").Official
			tt.change(&port)
			if err := filesApplyFZF(port, env); err == nil || string(testutil.Read(t, path)) != original {
				t.Fatalf("forged %s accepted or changed configuration: %v", tt.name, err)
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
			env := testutil.Env(t)
			filesFZFTestBinary(t, env, version)
			presets := filesFZFTestPresets
			filesFZFTestSource(t, map[string]*string{shared.FZFPresetsSource: &presets})
			if _, ok, err := filesResolveFZF(catalog.Port("catppuccin-mocha", "fzf"), env); ok || err == nil || !strings.Contains(err.Error(), "0.52.0") {
				t.Fatalf("incompatible version resolved: ok=%v, err=%v", ok, err)
			}
			path := shared.Path("fzf", env)
			original := "--color=fg:#010203\n"
			testutil.Write(t, path, original)
			if err := filesApplyFZF(*catalog.Port("catppuccin-mocha", "fzf").Official, env); err == nil || !strings.Contains(err.Error(), "0.52.0") {
				t.Fatal("incompatible version applied")
			}
			if string(testutil.Read(t, path)) != original {
				t.Fatal("theme target changed before version rejection")
			}
			// A custom options file does not inherit the official port's requirement.
			custom := filepath.Join(env.Home, "custom.opts")
			testutil.Write(t, custom, "--color=fg:#123456\n")
			if err := filesApplyFZF(model.Port{Kind: "custom", Name: "catppuccin-mocha", File: custom}, env); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"gruvbox", "nord"} {
				port, ok, err := filesResolveFZF(catalog.Port(name, "fzf"), env)
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
func TestTokyoNightAuthorFZFColorsOnly(t *testing.T) {
	const author = "export FZF_DEFAULT_OPTS=\"$FZF_DEFAULT_OPTS \\\n  --highlight-line \\\n  --ansi \\\n  --color=bg:#16161e \\\n  --color=hl+:#2ac3de \\\n  --color=query:#c0caf5:regular \\\n\"\n"
	colors, err := filesExtractFZFScriptColors([]byte(author))
	if err != nil {
		t.Fatal(err)
	}
	if string(colors) != "--color=bg:#16161e\n--color=hl+:#2ac3de\n--color=query:#c0caf5:regular\n" {
		t.Fatalf("unexpected author colors: %q", colors)
	}
	if _, err := filesExtractFZFScriptColors([]byte("--layout=reverse\n")); err == nil {
		t.Fatal("accepted a layout-only upstream script")
	}
	if _, err := filesExtractFZFScriptColors([]byte("--color=fg:#fff;touch /tmp/pwned\n")); err == nil {
		t.Fatal("accepted an executable shell argument")
	}
}

func TestRosePineFZFRejectsShellCode(t *testing.T) {
	for _, script := range []string{"--color=fg:#ffffff\n", "export FZF_DEFAULT_OPTS=\"\n --bind=execute(evil)\n\"", "export FZF_DEFAULT_OPTS=\"\n--color=fg:#ffffff\n--preview=evil\""} {
		if _, err := filesExtractFZFExportColors([]byte(script)); err == nil {
			t.Fatalf("accepted untrusted fzf script %q", script)
		}
	}
}

func TestFZFRejectsUnsafeOptions(t *testing.T) {
	for _, options := range []string{"--bind=enter:execute(rm -rf /)\n", "--color=fg:red --preview=bad\n", "$(touch /tmp/bad)\n", "--color='fg:red'\n"} {
		if err := filesFZFOptions([]byte(options)); err == nil {
			t.Errorf("unsafe/non-theme fzf options accepted: %q", options)
		}
	}
}

func TestFZFRemoteCustomUsesOnlyColorOptions(t *testing.T) {
	env := testutil.Env(t)
	if !Adapter().Remote {
		t.Fatal("fzf adapter does not accept remote themes")
	}
	const url = "https://example.test/fzf/colors"
	source := "# colors\n--color=fg:#aabbcc\n--color=bg:#112233\n"
	filesFZFTestSource(t, map[string]*string{url: &source})
	path := shared.Path("fzf", env)
	testutil.Write(t, path, "--color=fg:#000000\n")
	port := model.Port{Kind: "custom", Name: "my-theme", URL: url, Source: "author's theme"}
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	if string(testutil.Read(t, path)) != "--color=fg:#aabbcc\n--color=bg:#112233\n" {
		t.Fatal("remote colors were not applied")
	}
	before := testutil.Read(t, path)
	sentinel := filepath.Join(env.Home, "executed")
	source = "--color=fg:#aabbcc\n--bind=enter:execute(touch " + sentinel + ")\n"
	if err := Adapter().Apply(port, env); err == nil || !bytes.Equal(before, testutil.Read(t, path)) {
		t.Fatal("unsafe remote options changed fzf theme")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("remote shell code executed")
	}
}
