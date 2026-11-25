package adapter

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesOfficialNativeAssetsPreserveApplicationConfiguration(t *testing.T) {
	for _, tc := range []struct {
		id, name, existing, color string
	}{
		{"lazygit", "dracula", "gui:\n  sidePanelWidth: 0.42\n  mouseEvents: false\n  theme:\n    branchColorPatterns:\n      feature: blue\ngit:\n  autoFetch: false\nkeybinding:\n  universal:\n    quit: X\n", "#FF79C6"},
		{"lazygit", "catppuccin-mocha", "gui:\n  sidePanelWidth: 0.42\n  mouseEvents: false\n  theme:\n    branchColorPatterns:\n      feature: blue\ngit:\n  autoFetch: false\nkeybinding:\n  universal:\n    quit: X\n", "#cba6f7"},
		{"lazydocker", "dracula", "gui:\n  scrollHeight: 9\n  theme:\n    custom: keep\nlogs:\n  tail: '200'\n", "#FF79C6"},
		{"eza", "catppuccin-mocha", "filenames:\n  Makefile:\n    icon:\n      glyph: KEEP\nfilekinds:\n  directory:\n    is_italic: true\n", "#cba6f7"},
	} {
		t.Run(tc.id+"/"+tc.name, func(t *testing.T) {
			env := filesTestEnv(t)
			adapter := filesTestAdapter(t, tc.id)
			path := filesPath(tc.id, env)
			filesTestWrite(t, path, tc.existing)
			before, err := filesYAML([]byte(tc.existing))
			if err != nil {
				t.Fatal(err)
			}
			port, ok, err := adapter.Resolve(tc.name, env)
			if err != nil || !ok || port.Kind != "official" || !strings.Contains(port.Source, "/blob/") {
				t.Fatalf("official port unavailable: %#v, %v", port, err)
			}
			if err := adapter.Apply(port, env); err != nil {
				t.Fatal(err)
			}
			first := filesTestRead(t, path)
			after, err := filesYAML(first)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(first), tc.color) {
				t.Fatalf("official color missing: %s", first)
			}
			switch tc.id {
			case "lazygit":
				gui := after["gui"].(map[string]any)
				if gui["sidePanelWidth"] != before["gui"].(map[string]any)["sidePanelWidth"] || gui["mouseEvents"] != false || after["git"].(map[string]any)["autoFetch"] != false || after["keybinding"].(map[string]any)["universal"].(map[string]any)["quit"] != "X" || gui["theme"].(map[string]any)["branchColorPatterns"].(map[string]any)["feature"] != "blue" {
					t.Fatalf("unrelated lazygit configuration changed: %#v", after)
				}
				if _, exists := after["theme"]; exists {
					t.Fatal("upstream fragment was not placed under gui.theme")
				}
			case "lazydocker":
				gui := after["gui"].(map[string]any)
				if gui["scrollHeight"] != before["gui"].(map[string]any)["scrollHeight"] || gui["theme"].(map[string]any)["custom"] != "keep" || after["logs"].(map[string]any)["tail"] != "200" {
					t.Fatalf("unrelated lazydocker configuration changed: %#v", after)
				}
			case "eza":
				if after["filenames"].(map[string]any)["Makefile"].(map[string]any)["icon"].(map[string]any)["glyph"] != "KEEP" || after["filekinds"].(map[string]any)["directory"].(map[string]any)["is_italic"] != true {
					t.Fatalf("eza custom icon or unspecified style changed: %#v", after)
				}
			}
			if err := adapter.Apply(port, env); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, filesTestRead(t, path)) {
				t.Fatal("reapplying the official port is not idempotent")
			}
		})
	}
}

func TestFilesOfficialAtuinInstallsDeclaredThemeName(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(map[bool]string{false: "xdg", true: "modern"}[modern], func(t *testing.T) {
			env := filesTestEnv(t)
			path := filepath.Join(env.ConfigHome, "atuin", "config.toml")
			if modern {
				path = filepath.Join(env.Home, ".atuin", "config.toml")
			}
			filesTestWrite(t, path, "enter_accept = false\nsearch_mode = 'prefix'\n[theme]\ndebug = true\n")
			port, ok, err := filesResolveAtuin("catppuccin-mocha", env)
			if err != nil || !ok || port.Kind != "official" {
				t.Fatalf("official Atuin port unavailable: %#v, %v", port, err)
			}
			if err := filesApplyAtuin(port, env); err != nil {
				t.Fatal(err)
			}
			config, err := filesTOML(filesTestRead(t, path))
			if err != nil {
				t.Fatal(err)
			}
			theme := config["theme"].(map[string]any)
			if theme["name"] != "catppuccin-mocha-mauve" || theme["debug"] != true || config["enter_accept"] != false || config["search_mode"] != "prefix" {
				t.Fatalf("wrong Atuin selector or unrelated configuration changed: %#v", config)
			}
			asset := filesTestRead(t, filepath.Join(filepath.Dir(path), "themes", "catppuccin-mocha-mauve.toml"))
			if !bytes.Equal(asset, filesAtuinCatppuccin) {
				t.Fatal("installed Atuin theme differs from the official asset")
			}
		})
	}
}

func TestFilesOfficialYaziStylesPreserveLayoutAndInstallSyntaxTheme(t *testing.T) {
	env := filesTestEnv(t)
	env.Paths["yazi"] = filepath.Join(env.Home, "custom-yazi", "theme.toml")
	path := filesPath("yazi", env)
	filesTestWrite(t, path, "[flavor]\ndark = 'old'\nlight = 'old'\n[mgr]\nborder_symbol = 'KEEP'\n[status]\nsep_left = { open = 'LEFT', close = 'RIGHT' }\n[icon]\nprepend_dirs = [{ name = 'src', text = 'CUSTOM' }]\n[filetype]\nrules = [{ name = '*.mine', fg = 'red' }]\n")
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		filesTestWrite(t, filepath.Join(filepath.Dir(path), name), "# PRESERVE "+name+"\n")
	}
	port, ok, err := filesResolveYazi("catppuccin-mocha", env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("official Yazi port unavailable: %#v, %v", port, err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	first := filesTestRead(t, path)
	config, err := filesTOML(first)
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	if mgr["border_symbol"] != "KEEP" || mgr["cwd"].(map[string]any)["fg"] != "#94e2d5" || config["status"].(map[string]any)["sep_left"].(map[string]any)["open"] != "LEFT" || !strings.Contains(string(first), "CUSTOM") || !strings.Contains(string(first), "*.mine") {
		t.Fatalf("Yazi layout or overrides changed: %s", first)
	}
	flavor := config["flavor"].(map[string]any)
	if flavor["dark"] != "" || flavor["light"] != "" {
		t.Fatalf("old Yazi flavor still selected: %#v", flavor)
	}
	syntax := filepath.Join(filepath.Dir(path), "themeset-catppuccin-mocha.tmTheme")
	if mgr["syntect_theme"] != syntax || !bytes.Equal(filesTestRead(t, syntax), filesYaziCatppuccinSyntax) {
		t.Fatal("Yazi syntax preview does not reference the installed official asset")
	}
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		if string(filesTestRead(t, filepath.Join(filepath.Dir(path), name))) != "# PRESERVE "+name+"\n" {
			t.Fatal("Yazi application config or keymap changed")
		}
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatal("Yazi official reapply is not idempotent")
	}
}
