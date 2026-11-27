package tmux

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
)

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
	colors, err := filesTmuxStyles([]byte(script), *catalog.Port("tokyo-night", "tmux").Official)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(colors), "status-left") || strings.Contains(string(colors), "status-right-length") || strings.Contains(string(colors), "if-shell") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("author layout or shell code leaked into tmux theme: %s", colors)
	}
	if !strings.Contains(string(colors), "pane-active-border-style") || !strings.Contains(string(colors), "window-status-style") {
		t.Fatalf("author color styles missing: %s", colors)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-left \"layout only\""), *catalog.Port("tokyo-night", "tmux").Official); err == nil {
		t.Fatal("accepted a layout-only tmux script")
	}
}

func TestMoonflyTmuxUsesDeclaredStyleKeys(t *testing.T) {
	port := *catalog.Port("moonfly", "tmux").Official
	const script = "set -g pane-active-border-style \"fg=#111111\"\nset -g status-style \"bg=#222222\"\nset -g window-status-current-style none\nset -g mode-style \"fg=#333333\"\nrun-shell 'false'\n"
	colors, err := filesTmuxStyles([]byte(script), port)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(colors), "window-status-current-style none") || strings.Contains(string(colors), "mode-style") || strings.Contains(string(colors), "run-shell") {
		t.Fatalf("styles escaped declared allowlist: %s", colors)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-style \"bg=#222222\"\n"), port); err == nil {
		t.Fatal("accepted a source missing the required pane border color")
	}
}

func TestNightfoxTmuxImportsStylesWithoutReplacingStatusLayout(t *testing.T) {
	port := *catalog.Port("nightfox", "tmux").Official
	const source = `#!/usr/bin/env bash
set -g pane-active-border-style "fg=#719cd6"
set -g status-style "fg=#aeafb0,bg=#131a24"
setw -g window-status-style "NONE,fg=#71839b,bg=#131a24"
set -g status-left "#[fg=#131a24] #S"
setw -g window-status-current-format "#[fg=#aeafb0] #I"
set -g status-right-length "100"
`
	styles, err := filesTmuxStyles([]byte(source), port)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(styles), "pane-active-border-style") ||
		!strings.Contains(string(styles), "window-status-style") ||
		strings.Contains(string(styles), "status-left") ||
		strings.Contains(string(styles), "window-status-current-format") ||
		strings.Contains(string(styles), "status-right-length") {
		t.Fatalf("nightfox style selection changed status layout: %s", styles)
	}
	if _, err := filesTmuxStyles([]byte("set -g status-left \"layout only\"\n"), port); err == nil {
		t.Fatal("accepted Nightfox source without required style colors")
	}
}

func TestCatppuccinTmuxRequiresInstalledPluginAndSwitchesCleanly(t *testing.T) {
	env := testutil.Env(t)
	bin := t.TempDir()
	testutil.Write(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nprintf 'tmux 3.7c\\n'\n")
	if err := os.Chmod(filepath.Join(bin, "tmux"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	entry := filepath.Join(env.ConfigHome, "tmux", "plugins", "catppuccin", "tmux", "catppuccin.tmux")
	root := filepath.Dir(entry)
	for _, flavor := range []string{"latte", "frappe", "macchiato", "mocha"} {
		name := "catppuccin-" + flavor
		spec := catalog.Port(name, "tmux")
		if port, ok, err := nativeResolveTmux(spec, env); !ok || err != nil || port.File != entry {
			t.Fatalf("%s not planned for installation: %+v, %v, %v", name, port, ok, err)
		}
	}
	for _, path := range []string{entry, filepath.Join(root, "catppuccin_options_tmux.conf"), filepath.Join(root, "catppuccin_tmux.conf")} {
		testutil.Write(t, path, "# plugin file\n")
	}
	if port, ok, err := nativeResolveTmux(catalog.Port("catppuccin-frappe", "tmux"), env); !ok || err != nil || port.File != entry {
		t.Fatalf("incomplete flavor not planned: %+v, %v, %v", port, ok, err)
	}
	for _, flavor := range []string{"latte", "frappe", "macchiato", "mocha"} {
		name := "catppuccin-" + flavor
		testutil.Write(t, filepath.Join(root, "themes", "catppuccin_"+flavor+"_tmux.conf"), "# official flavor\n")
		port, ok, err := nativeResolveTmux(catalog.Port(name, "tmux"), env)
		if flavor == "latte" {
			testutil.Write(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nprintf 'tmux 3.1\\n'\n")
			if err := nativeApplyTmux(port, env); err == nil || !strings.Contains(err.Error(), "requires tmux 3.2") {
				t.Fatalf("unsupported tmux version accepted: %v", err)
			}
			testutil.Write(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nprintf 'tmux 3.7c\\n'\n")
		}
		if err != nil || !ok || port.Kind != "native" || port.Name != name || port.File != entry {
			t.Fatalf("%s did not select installed flavor: %#v, %v, %v", name, port, ok, err)
		}
		if err := nativeApplyTmux(port, env); err != nil {
			t.Fatal(err)
		}
		selector := string(testutil.Read(t, filepath.Join(env.ConfigHome, "themeset", "tmux-"+name+".conf")))
		if !strings.Contains(selector, "@catppuccin_flavor \""+flavor+"\"") ||
			!strings.Contains(selector, "run-shell \"'"+entry+"'\"") ||
			!strings.Contains(selector, "@catppuccin_status_session") ||
			!strings.Contains(selector, "@catppuccin_status_application") {
			t.Fatalf("%s does not activate upstream layout: %q", name, selector)
		}
		config := string(testutil.Read(t, filepath.Join(env.Home, ".tmux.conf")))
		if strings.Count(config, "# themeset") != 1 || !strings.Contains(config, "tmux-"+name+".conf") {
			t.Fatalf("%s left obsolete theme source: %q", name, config)
		}
	}
	// Switching away removes the executable plugin from the active include.
	style := *catalog.Port("moonfly", "tmux").Official
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != style.Source {
			t.Fatalf("unexpected source %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("set -g pane-active-border-style none\nset -g status-style none\n"))}, nil
	})
	if err := nativeApplyTmux(style, env); err != nil {
		t.Fatal(err)
	}
	config := string(testutil.Read(t, filepath.Join(env.Home, ".tmux.conf")))
	if strings.Contains(config, "catppuccin-") || !strings.Contains(config, "tmux-moonfly.conf") {
		t.Fatalf("plugin remained active after switching: %s", config)
	}
}

func TestInstalledUpstreamTmuxThemesSelectTheirNativeVariants(t *testing.T) {
	env := testutil.Env(t)
	for _, theme := range []string{"rose-pine", "rose-pine-dawn", "rose-pine-moon", "dracula", "nord", "gruvbox", "everforest"} {
		plugin, registered := tmuxPlugin(theme)
		if !registered {
			t.Fatalf("unregistered plugin %s", theme)
		}
		entry := filepath.Join(env.ConfigHome, "tmux", "plugins", plugin.repo, plugin.entry)
		for _, path := range append([]string{plugin.entry}, plugin.required...) {
			testutil.Write(t, filepath.Join(filepath.Dir(entry), path), "# upstream file\n")
		}
		port, ok, err := nativeResolveTmux(catalog.Port(theme, "tmux"), env)
		if err != nil || !ok || port.Kind != "native" || port.File != entry || port.Name != theme {
			t.Fatalf("%s selected wrong port: %+v, %v, %v", theme, port, ok, err)
		}
		if err := nativeApplyTmux(port, env); err != nil {
			t.Fatal(err)
		}
		selector := string(testutil.Read(t, filepath.Join(env.ConfigHome, "themeset", "tmux-"+theme+".conf")))
		if !strings.Contains(selector, "run-shell \"'"+entry+"'\"") ||
			plugin.option != "" && !strings.Contains(selector, "set -g "+plugin.option+" \""+plugin.variant+"\"") {
			t.Fatalf("%s wrong upstream variant: %q", theme, selector)
		}
		config := string(testutil.Read(t, filepath.Join(env.Home, ".tmux.conf")))
		if strings.Count(config, "# themeset") != 1 || !strings.Contains(config, "tmux-"+theme+".conf") {
			t.Fatalf("%s stale managed include: %s", theme, config)
		}
	}
}

func TestTmuxPluginInstallIsStagedAndPreservesConfigOnFailure(t *testing.T) {
	env := testutil.Env(t)
	entry := filepath.Join(env.ConfigHome, "tmux", "plugins", "rose-pine", "tmux", "rose-pine.tmux")
	port, ok, err := nativeResolveTmux(catalog.Port("rose-pine-dawn", "tmux"), env)
	if !ok || err != nil || port.File != entry {
		t.Fatalf("missing plugin not planned: %+v, %v", port, err)
	}
	testutil.Write(t, filepath.Join(env.Home, ".tmux.conf"), "set -g mouse on\n")
	bin := t.TempDir()
	git := filepath.Join(bin, "git")
	testutil.Write(t, git, "#!/bin/sh\nfor target do :; done\ncp \"$THEMESET_PLUGIN_FIXTURE\" \"$target/rose-pine.tmux\"\n")
	if err := os.Chmod(git, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	fixture := filepath.Join(t.TempDir(), "rose-pine.tmux")
	testutil.Write(t, fixture, "# upstream plugin\n")
	t.Setenv("THEMESET_PLUGIN_FIXTURE", fixture)
	if err := nativeApplyTmux(port, env); err != nil {
		t.Fatal(err)
	}
	if got := string(testutil.Read(t, entry)); got != "# upstream plugin\n" {
		t.Fatalf("plugin not installed: %q", got)
	}
	config := string(testutil.Read(t, filepath.Join(env.Home, ".tmux.conf")))
	if !strings.Contains(config, "set -g mouse on") || !strings.Contains(config, "tmux-rose-pine-dawn.conf") {
		t.Fatalf("existing config not preserved: %s", config)
	}
	// An installed plugin must be reused rather than downloaded again.
	testutil.Write(t, git, "#!/bin/sh\nexit 1\n")
	if err := nativeApplyTmux(port, env); err != nil {
		t.Fatalf("reused plugin failed: %v", err)
	}

	failing := testutil.Env(t)
	testutil.Write(t, filepath.Join(failing.Home, ".tmux.conf"), "set -g mouse on\n")
	port, ok, err = nativeResolveTmux(catalog.Port("rose-pine", "tmux"), failing)
	if !ok || err != nil {
		t.Fatalf("missing plugin not planned: %+v, %v", port, err)
	}
	if err := nativeApplyTmux(port, failing); err == nil {
		t.Fatal("clone failure reported success")
	}
	if got := string(testutil.Read(t, filepath.Join(failing.Home, ".tmux.conf"))); got != "set -g mouse on\n" {
		t.Fatalf("failed install changed config: %s", got)
	}
	if _, err := os.Stat(port.File); !os.IsNotExist(err) {
		t.Fatalf("failed install left plugin entry: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(filepath.Dir(port.File)))
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed install left staged files: %v, %v", entries, err)
	}
	testutil.Write(t, git, "#!/bin/sh\nexit 0\n")
	if err := nativeApplyTmux(port, failing); err == nil || !strings.Contains(err.Error(), "lacks required files") {
		t.Fatalf("incomplete download accepted: %v", err)
	}
	if got := string(testutil.Read(t, filepath.Join(failing.Home, ".tmux.conf"))); got != "set -g mouse on\n" {
		t.Fatalf("incomplete download changed config: %s", got)
	}
	entries, err = os.ReadDir(filepath.Dir(filepath.Dir(port.File)))
	if err != nil || len(entries) != 0 {
		t.Fatalf("incomplete download left staged files: %v, %v", entries, err)
	}
	marker := filepath.Join(filepath.Dir(port.File), "user.txt")
	testutil.Write(t, marker, "keep\n")
	if err := nativeApplyTmux(port, failing); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete user install replaced: %v", err)
	}
	if got := string(testutil.Read(t, marker)); got != "keep\n" {
		t.Fatalf("incomplete plugin overwritten: %q", got)
	}
}
