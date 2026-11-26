package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestTokyoNightOfficialRoutesReadOnly(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"btop", "eza", "fzf", "lazygit", "yazi"} {
		port, ok, err := filesResolveTokyoNightPort(id, "tokyo-night", env)
		if err != nil || !ok || port.Kind != "official" || port.Source != filesTokyoNightBase+filesTokyoNightAssets[id].path {
			t.Fatalf("%s: port=%#v ok=%v err=%v", id, port, ok, err)
		}
		if id == "btop" && port.Name != "tokyonight_night" {
			t.Fatalf("btop must select upstream filename: %#v", port)
		}
		if id != "btop" && port.Name != "tokyo-night" {
			t.Fatalf("%s unexpected theme name: %#v", id, port)
		}
		if _, ok, err := filesResolveTokyoNightPort(id, "tokyonight", env); ok || err != nil {
			t.Fatalf("%s unexpectedly registered an additional catalog ID", id)
		}
	}
	for _, id := range []string{"atuin", "starship", "lazydocker", "syntax"} {
		if _, ok, err := filesResolveTokyoNightPort(id, "tokyo-night", env); ok || err != nil {
			t.Fatalf("unsupported %s port resolved", id)
		}
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolution touched cache: %v", err)
	}
}

func TestTokyoNightAuthorFZFColorsOnly(t *testing.T) {
	const author = "export FZF_DEFAULT_OPTS=\"$FZF_DEFAULT_OPTS \\\n  --highlight-line \\\n  --ansi \\\n  --color=bg:#16161e \\\n  --color=hl+:#2ac3de \\\n  --color=query:#c0caf5:regular \\\n\"\n"
	colors, err := filesTokyoNightFZF([]byte(author))
	if err != nil {
		t.Fatal(err)
	}
	if string(colors) != "--color=bg:#16161e\n--color=hl+:#2ac3de\n--color=query:#c0caf5:regular\n" {
		t.Fatalf("unexpected author colors: %q", colors)
	}
	if _, err := filesTokyoNightFZF([]byte("--layout=reverse\n")); err == nil {
		t.Fatal("accepted a layout-only upstream script")
	}
	if _, err := filesTokyoNightFZF([]byte("--color=fg:#fff;touch /tmp/pwned\n")); err == nil {
		t.Fatal("accepted an executable shell argument")
	}
}

func TestTokyoNightYaziInstallsAuthorFlavorWithoutOverwritingLayout(t *testing.T) {
	env := filesTestEnv(t)
	path := filesPath("yazi", env)
	filesTestWrite(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 2, 3]\ncwd = { italic = true }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	filesTestWrite(t, filepath.Join(filepath.Dir(path), "yazi.toml"), "# untouched\n")
	const flavor = "[mgr]\ncwd = { fg = '#a9b1d6' }\n[filetype]\nrules = [{ name = '*', fg = '#c0caf5' }]\n"
	const syntax = "<?xml version=\"1.0\"?><plist><dict/></plist>"
	if err := filesInstallTokyoNightYazi(env, []byte(flavor), []byte(syntax)); err != nil {
		t.Fatal(err)
	}
	root := filesYaziFlavorRoot("tokyo-night", env)
	if string(filesTestRead(t, filepath.Join(root, "flavor.toml"))) != flavor || string(filesTestRead(t, filepath.Join(root, "tmtheme.xml"))) != syntax {
		t.Fatal("installed flavor differs from verified author files")
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	selection := config["flavor"].(map[string]any)
	if selection["dark"] != "tokyo-night" || selection["light"] != "tokyo-night" || selection["use"] != nil || config["mgr"].(map[string]any)["ratio"] == nil || config["icon"] == nil {
		t.Fatalf("selection discarded unrelated settings: %#v", config)
	}
	if got := string(filesTestRead(t, filepath.Join(filepath.Dir(path), "yazi.toml"))); got != "# untouched\n" {
		t.Fatalf("rewrote yazi.toml: %q", got)
	}
	first := filesTestRead(t, path)
	if err := filesInstallTokyoNightYazi(env, []byte(flavor), []byte(syntax)); err != nil || !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatalf("flavor reapplication changed selection: %v", err)
	}
}

func TestTokyoNightYaziRejectsInvalidSyntaxBeforeWriting(t *testing.T) {
	env := filesTestEnv(t)
	path := filesPath("yazi", env)
	filesTestWrite(t, path, "[flavor]\ndark = 'old'\n")
	for _, bad := range []string{"", "<not-a-plist/>", "<plist>"} {
		if err := filesInstallTokyoNightYazi(env, []byte("[mgr]\ncwd = { fg = '#a9b1d6' }\n"), []byte(bad)); err == nil {
			t.Fatalf("accepted invalid syntax %q", bad)
		}
		if string(filesTestRead(t, path)) != "[flavor]\ndark = 'old'\n" {
			t.Fatal("bad syntax changed active theme")
		}
	}
}

func TestTokyoNightAcquisitionFailurePreservesActiveThemes(t *testing.T) {
	for _, id := range []string{"eza", "lazygit", "fzf", "btop", "yazi"} {
		t.Run(id, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath(id, env)
			const original = "# untouched\n"
			filesTestWrite(t, path, original)
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline: %s", r.URL) })
			port, ok, err := filesResolveTokyoNightPort(id, "tokyo-night", env)
			if err != nil || !ok {
				t.Fatal("official port missing")
			}
			switch id {
			case "eza":
				err = filesApplyEza(port, env)
			case "lazygit":
				err = filesApplyLazy(id)(port, env)
			case "fzf":
				err = filesApplyFZF(port, env)
			case "btop":
				err = filesApplyBtop(port, env)
			case "yazi":
				err = filesApplyYazi(port, env)
			}
			if err == nil || !strings.Contains(err.Error(), "offline") {
				t.Fatalf("expected acquisition failure, got %v", err)
			}
			if string(filesTestRead(t, path)) != original {
				t.Fatal("failed acquisition changed existing config")
			}
		})
	}
}

func TestTokyoNightRejectsUnregisteredOfficialSources(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"eza", "fzf", "btop", "lazygit", "yazi"} {
		port, _, _ := filesResolveTokyoNightPort(id, "tokyo-night", env)
		port.Source += "?unverified=true"
		if _, err := filesAcquireTokyoNightPort(id, port, env); err == nil {
			t.Fatalf("accepted unregistered %s source", id)
		}
	}
	btop, _, _ := filesResolveTokyoNightPort("btop", "tokyo-night", env)
	btop.Name = "tokyo-night"
	if _, err := filesAcquireTokyoNightPort("btop", btop, env); err == nil {
		t.Fatal("accepted wrong native btop filename")
	}
	if _, err := filesAcquireTokyoNightPort("starship", model.Port{Kind: "official", Name: "tokyo-night"}, env); err == nil {
		t.Fatal("accepted unsupported Starship source")
	}
}

func TestTokyoNightTmuxKeepsOnlyAuthorColors(t *testing.T) {
	const script = `#!/usr/bin/env bash
set -g mode-style "fg=#7aa2f7,bg=#3b4261"
set -g pane-active-border-style "fg=#7aa2f7"
set -g status-style "fg=#7aa2f7,bg=#16161e"
set -g status-left "#[bg=#7aa2f7] unrelated user layout"
set -g status-right-length "100"
setw -g window-status-style "NONE,fg=#a9b1d6,bg=#16161e"
if-shell 'echo should-not-run' { run-shell 'false' }
`
	colors, err := filesTokyoNightTmuxColors([]byte(script))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(colors), "status-left") || strings.Contains(string(colors), "status-right-length") || strings.Contains(string(colors), "if-shell") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("author layout or shell code leaked into tmux theme: %s", colors)
	}
	if !strings.Contains(string(colors), "pane-active-border-style") || !strings.Contains(string(colors), "window-status-style") {
		t.Fatalf("author color styles missing: %s", colors)
	}
	if _, err := filesTokyoNightTmuxColors([]byte("set -g status-left \"layout only\"")); err == nil {
		t.Fatal("accepted a layout-only tmux script")
	}
}

func TestTokyoNightTmuxAcquisitionFailureKeepsConfig(t *testing.T) {
	env := filesTestEnv(t)
	path := filepath.Join(env.Home, ".tmux.conf")
	filesTestWrite(t, path, "set -g status-left 'my layout'\n")
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline: %s", r.URL)
	})
	port, ok, err := nativeResolveTmux("tokyo-night", env)
	if err != nil || !ok || port.Kind != "official" || port.Source != filesTokyoNightBase+filesTokyoNightAssets["tmux"].path {
		t.Fatalf("Tokyo Night tmux not registered: %#v, %v", port, err)
	}
	if err := nativeApplyTmux(port, env); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Fatalf("expected safe failed acquisition, got %v", err)
	}
	if got := string(filesTestRead(t, path)); got != "set -g status-left 'my layout'\n" {
		t.Fatalf("acquisition rewrote tmux layout: %q", got)
	}
}

func TestTokyoNightNativeFlavorsTakePrecedenceOverDownloads(t *testing.T) {
	env := filesTestEnv(t)
	const flavor = "[mgr]\ncwd = { fg = '#abcdef' }\n"
	const syntax = "<plist><dict/></plist>"
	root := filesYaziFlavorRoot("tokyo-night", env)
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), flavor)
	filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), syntax)
	port, ok, err := filesResolveYazi("tokyo-night", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") {
		t.Fatalf("installed native Tokyo Night Yazi port ignored: %#v, %v", port, err)
	}
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("native port must not download: %s", r.URL)
		return nil, fmt.Errorf("unexpected download")
	})
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if string(filesTestRead(t, filepath.Join(root, "flavor.toml"))) != flavor || string(filesTestRead(t, filepath.Join(root, "tmtheme.xml"))) != syntax {
		t.Fatal("replaced an independently installed flavor")
	}
	config, err := filesTOML(filesTestRead(t, filesPath("yazi", env)))
	if err != nil || config["flavor"].(map[string]any)["dark"] != "tokyo-night" {
		t.Fatalf("native Yazi flavor not selected: %#v %v", config, err)
	}

	btopPath := filepath.Join(filepath.Dir(filesPath("btop", env)), "themes", "tokyonight_night.theme")
	filesTestWrite(t, btopPath, "theme[main_bg]=\"#1a1b26\"\n")
	btop, ok, err := filesResolveBtop("tokyo-night", env)
	if err != nil || !ok || btop.Kind != "native" || btop.File != btopPath || btop.Name != "tokyonight_night" {
		t.Fatalf("installed native btop port ignored: %#v, %v", btop, err)
	}
	if err := filesApplyBtop(btop, env); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(filesTestRead(t, filesPath("btop", env))), "tokyonight_night") {
		t.Fatal("native btop theme was not selected")
	}
}

func TestTokyoNightOfficialYaziCannotReplaceExistingFlavor(t *testing.T) {
	env := filesTestEnv(t)
	root := filesYaziFlavorRoot("tokyo-night", env)
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#abcdef' }\n")
	if err := filesInstallTokyoNightYazi(env, []byte("[mgr]\ncwd = { fg = '#a9b1d6' }\n"), []byte("<plist><dict/></plist>")); err == nil {
		t.Fatal("replaced an independently installed partial flavor")
	}
	if got := string(filesTestRead(t, filepath.Join(root, "flavor.toml"))); !strings.Contains(got, "#abcdef") {
		t.Fatalf("existing flavor changed: %q", got)
	}
}
