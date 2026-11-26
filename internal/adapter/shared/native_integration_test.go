package shared_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/bat"
	"github.com/clang-engineer/themeset/internal/adapter/delta"
	"github.com/clang-engineer/themeset/internal/adapter/ghostty"
	"github.com/clang-engineer/themeset/internal/adapter/harlequin"
	"github.com/clang-engineer/themeset/internal/adapter/neovim"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/tmux"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

func nativeTestEnvironment(t *testing.T) model.Environment {
	t.Helper()
	// macOS's temporary directory may be reached through /var -> /private/var.
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return model.Environment{Home: home, ConfigHome: filepath.Join(home, "config"), OS: "linux", Paths: map[string]string{"bat": filepath.Join(home, "config", "bat", "config")}}
}

func nativeTestPut(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func nativeTestGet(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type nativeTransport func(*http.Request) (*http.Response, error)

func (transport nativeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func nativeAssetClient(t *testing.T, transport nativeTransport) {
	t.Helper()
	previous := shared.AssetHTTPClient
	shared.AssetHTTPClient = &http.Client{Transport: transport}
	t.Cleanup(func() { shared.AssetHTTPClient = previous })
}

func TestNativeListedResolverRequiresExactInstalledTheme(t *testing.T) {
	env := nativeTestEnvironment(t)
	bin := filepath.Join(env.Home, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "ghostty")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s\\n' 'Dracula Plus (dark)' 'Nord (dark)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	resolver := ghostty.Adapter().Resolve
	if _, ok, err := resolver("dracula", env); err != nil || ok {
		t.Fatalf("uninstalled exact theme: ok=%v err=%v", ok, err)
	}
	port, ok, err := resolver("nord", env)
	if err != nil || !ok || port.Name != "Nord" || port.Source == "" {
		t.Fatalf("installed theme: %#v %v %v", port, ok, err)
	}
	if _, ok, err := resolver("made-up-palette", env); err != nil || ok {
		t.Fatalf("unsupported theme: %v %v", ok, err)
	}
}

func TestNativeGhosttyPreservesOtherSettings(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.ConfigHome, "ghostty", "config")
	nativeTestPut(t, path, "# user comment\nfont-size = 14\ntheme = Old\nkeybind = ctrl+a=new_tab\ntheme = Duplicate\n")
	if err := ghostty.Adapter().Apply(model.Port{Name: "Nord"}, env); err != nil {
		t.Fatal(err)
	}
	result := nativeTestGet(t, path)
	for _, want := range []string{"# user comment", "font-size = 14", "keybind = ctrl+a=new_tab", "theme = \"Nord\""} {
		if !strings.Contains(result, want) {
			t.Fatalf("missing %q in %s", want, result)
		}
	}
	if strings.Count(result, "theme =") != 1 {
		t.Fatalf("duplicate selections: %s", result)
	}
	asset := filepath.Join(env.Home, "themes", "trusted")
	nativeTestPut(t, asset, "background = #000000\n")
	if err := ghostty.Adapter().Apply(model.Port{File: asset}, env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nativeTestGet(t, path), asset) {
		t.Fatal("custom theme asset was not selected")
	}
}

func TestNativeBatMergesOnlyThemeSelection(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.ConfigHome, "bat", "config")
	source := filepath.Join(env.Home, "bat-source")
	nativeTestPut(t, path, "--style=numbers\n--theme='Old'\n--paging=never\n")
	nativeTestPut(t, source, "--theme=\"Catppuccin Mocha\"\n--paging=always\n")
	if err := bat.Adapter().Apply(model.Port{File: source}, env); err != nil {
		t.Fatal(err)
	}
	result := nativeTestGet(t, path)
	if !strings.Contains(result, "--style=numbers") || !strings.Contains(result, "--paging=never") || strings.Contains(result, "--paging=always") || !strings.Contains(result, "--theme=\"Catppuccin Mocha\"") {
		t.Fatal(result)
	}
}

func TestNativeTmuxUsesTrustedIncludeWithoutChangingLayout(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.Home, ".tmux.conf")
	source := filepath.Join(env.Home, "theme file.conf")
	nativeTestPut(t, path, "set -g status-left 'user layout'\n")
	nativeTestPut(t, source, "set -g status-style 'fg=white,bg=black'\n")
	if err := tmux.Adapter().Apply(model.Port{File: source}, env); err != nil {
		t.Fatal(err)
	}
	if err := tmux.Adapter().Apply(model.Port{File: source}, env); err != nil {
		t.Fatal(err)
	}
	result := nativeTestGet(t, path)
	if !strings.Contains(result, "set -g status-left 'user layout'") || strings.Count(result, "# themeset") != 1 {
		t.Fatal(result)
	}
	if err := tmux.Adapter().Apply(model.Port{Name: "dracula"}, env); err == nil {
		t.Fatal("fabricated tmux native theme accepted")
	}
}

func TestNativeOfficialMoonflyTmuxPreservesLayoutsAndBindings(t *testing.T) {
	for _, managed := range []bool{false, true} {
		name := "ordinary config"
		if managed {
			name = "Oh my tmux wrapper"
		}
		t.Run(name, func(t *testing.T) {
			env := nativeTestEnvironment(t)
			path := filepath.Join(env.Home, ".tmux.conf")
			layout := "set -g status-left 'KEEP LEFT'\nset -g status-right 'KEEP RIGHT'\nset -g mouse on\nbind r source-file ~/.tmux.conf\n"
			wrapper := ""
			if managed {
				env.Paths["tmux"] = filepath.Join(env.ConfigHome, "themeset", "tmux.conf")
				wrapper = "source-file ~/.tmux/.tmux.conf\nsource-file -q \"" + env.Paths["tmux"] + "\"\n" + layout
				nativeTestPut(t, path, wrapper)
				path = env.Paths["tmux"]
			}
			nativeTestPut(t, path, layout)
			script := "set -g pane-active-border-style \"fg=#111111\"\nset -g status-style \"bg=#222222\"\nset -g status-left 'OVERRIDE'\nrun-shell 'false'\n"
			nativeAssetClient(t, func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(script)), Header: make(http.Header)}, nil
			})
			port, ok, err := tmux.Adapter().Resolve("moonfly", env)
			if err != nil || !ok || port.Kind != "official" || port.Source != shared.MoonflySource+"moonfly.tmux" {
				t.Fatalf("missing current official Moonfly: %#v, %v", port, err)
			}
			if err := tmux.Adapter().Apply(port, env); err != nil {
				t.Fatal(err)
			}
			first := nativeTestGet(t, path)
			source := filepath.Join(env.ConfigHome, "themeset", "tmux-moonfly.conf")
			if !strings.HasPrefix(first, layout) || !strings.Contains(first, source) || strings.Count(first, "# themeset") != 1 {
				t.Fatalf("layout/bindings changed: %s", first)
			}
			styles := nativeTestGet(t, source)
			if !strings.Contains(styles, "pane-active-border-style \"fg=#111111\"") ||
				!strings.Contains(styles, "status-style \"bg=#222222\"") ||
				strings.Contains(styles, "status-left") || strings.Contains(styles, "run-shell") {
				t.Fatalf("Moonfly style extraction changed layout or omitted colors: %s", styles)
			}
			script = "set -g pane-active-border-style \"fg=#333333\"\nset -g status-style \"bg=#444444\"\n"
			if err := tmux.Adapter().Apply(port, env); err != nil {
				t.Fatal(err)
			}
			if nativeTestGet(t, path) != first || !strings.Contains(nativeTestGet(t, source), "fg=#333333") {
				t.Fatal("reapply failed to refresh styles or changed tmux include")
			}
			if managed && nativeTestGet(t, filepath.Join(env.Home, ".tmux.conf")) != wrapper {
				t.Fatal("Oh my tmux wrapper was changed")
			}
		})
	}
}

func TestNativeNeovimRequiresInstalledRuntimeAndOwnsOnlyDedicatedPlugin(t *testing.T) {
	env := nativeTestEnvironment(t)
	if _, ok, err := neovim.Adapter().Resolve("dracula", env); err != nil || ok {
		t.Fatalf("missing runtime: %v %v", ok, err)
	}
	root := filepath.Join(env.Home, ".local", "share", "nvim", "lazy", "dracula")
	nativeTestPut(t, filepath.Join(root, "colors", "dracula.vim"), "let g:colors_name = 'dracula'\n")
	port, ok, err := neovim.Adapter().Resolve("dracula", env)
	if err != nil || !ok {
		t.Fatalf("installed runtime: %v %v", ok, err)
	}
	if err := neovim.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(env.Home, "existing.lua")
	nativeTestPut(t, override, "return 'do not replace'\n")
	env.Paths["neovim"] = override
	if err := neovim.Adapter().Apply(port, env); err == nil {
		t.Fatal("overwrote unrelated Lua file")
	}
	if nativeTestGet(t, override) != "return 'do not replace'\n" {
		t.Fatal("unrelated file changed")
	}
}

func TestNativeHarlequinPreservesConnectionAndKeymapFields(t *testing.T) {
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.ConfigHome, "harlequin", "harlequin.toml")
	nativeTestPut(t, path, "default_profile = 'prod'\n[profiles.prod]\nadapter = 'postgres'\nhost = 'db.example'\ntheme = 'old'\n[profiles.local]\nadapter = 'duckdb'\nconn_str = ['safe.db']\n[[keymaps.user]]\nkeys = 'ctrl+s'\naction = 'save'\n")
	if err := harlequin.Adapter().Apply(model.Port{Name: "nord"}, env); err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := toml.Unmarshal([]byte(nativeTestGet(t, path)), &document); err != nil {
		t.Fatal(err)
	}
	profiles := document["profiles"].(map[string]any)
	prod := profiles["prod"].(map[string]any)
	if prod["theme"] != "nord" || prod["host"] != "db.example" || prod["adapter"] != "postgres" || document["default_profile"] != "prod" || document["keymaps"] == nil {
		t.Fatal(document)
	}
	if profiles["local"].(map[string]any)["theme"] != "nord" {
		t.Fatal(document)
	}
}

func TestNativeDeltaPreservesGitAndNavigationSettings(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for its native config parser")
	}
	env := nativeTestEnvironment(t)
	path := filepath.Join(env.Home, ".gitconfig")
	nativeTestPut(t, path, "[user]\n name = User\n[delta]\n line-numbers = true\n syntax-theme = Old\n[include]\n path = private.gitconfig\n")
	source := filepath.Join(env.Home, "delta-theme.gitconfig")
	nativeTestPut(t, source, "[delta]\n syntax-theme = Dracula\n minus-style = syntax auto\n line-numbers = false\n[user]\n name = Do Not Copy\n")
	if err := delta.Adapter().Apply(model.Port{File: source}, env); err != nil {
		t.Fatal(err)
	}
	result := nativeTestGet(t, path)
	for _, want := range []string{"name = User", "line-numbers = true", "syntax-theme = Dracula", "private.gitconfig", "minus-style = syntax auto"} {
		if !strings.Contains(result, want) {
			t.Fatalf("missing %q: %s", want, result)
		}
	}
	if strings.Contains(result, "Do Not Copy") {
		t.Fatal(result)
	}
}
