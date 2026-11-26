package adapter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

const filesOfficialTestLazy = "theme:\n  activeBorderColor: ['#cba6f7', bold]\n"
const filesOfficialTestDocker = "gui:\n  theme:\n    activeBorderColor: ['#FF79C6', bold]\n"
const filesOfficialTestAtuin = "[theme]\nname = 'catppuccin-mocha-mauve'\n[colors]\nAlertInfo = '#cba6f7'\n"
const filesOfficialTestYazi = "[mgr]\ncwd = { fg = '#94e2d5' }\nfind_keyword = { fg = '#f9e2af' }\nborder_symbol = 'OVERRIDE'\n[status]\nsep_left = { open = 'REPLACE' }\n"
const filesOfficialTestSyntax = "<?xml version=\"1.0\"?><plist><dict/></plist>"

func filesOfficialTestClient(t *testing.T) {
	t.Helper()
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		var body string
		switch r.URL.String() {
		case filesYaziCatppuccinSyntaxSource:
			body = filesOfficialTestSyntax
		default:
			for _, id := range []string{"lazygit", "lazydocker", "atuin", "eza", "yazi"} {
				name := "catppuccin-mocha"
				if id == "lazydocker" {
					name = "dracula"
				}
				if id == "lazygit" && r.URL.String() == filesOfficialTestSource("lazygit", "dracula") {
					name = "dracula"
				}
				if r.URL.String() != filesOfficialTestSource(id, name) {
					continue
				}
				switch id {
				case "lazygit":
					body = filesOfficialTestLazy
					if name == "dracula" {
						body = strings.ReplaceAll(body, "#cba6f7", "#FF79C6")
					}
				case "lazydocker":
					body = filesOfficialTestDocker
				case "atuin":
					body = filesOfficialTestAtuin
				case "eza":
					body = strings.ReplaceAll(strings.ReplaceAll(ezaDraculaTestTheme, "#8BE9FD", "#cba6f7"), `foreground: "F1FA8C"`, `foreground: "#F1FA8C"`)
				case "yazi":
					body = filesOfficialTestYazi
				}
				break
			}
			if body == "" {
				return nil, fmt.Errorf("unexpected official source %s", r.URL)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
}

func filesOfficialTestSource(id, name string) string {
	port, ok := filesOfficialPort(id, name)
	if !ok {
		return ""
	}
	return port.Source
}

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
			filesOfficialTestClient(t)
			env := filesTestEnv(t)
			adapter := filesTestAdapter(t, tc.id)
			path := filesPath(tc.id, env)
			filesTestWrite(t, path, tc.existing)
			before, err := filesYAML([]byte(tc.existing))
			if err != nil {
				t.Fatal(err)
			}
			port, ok, err := adapter.Resolve(tc.name, env)
			if err != nil || !ok || port.Kind != "official" || !strings.HasPrefix(port.Source, "https://raw.githubusercontent.com/") {
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
			filesOfficialTestClient(t)
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
			if !bytes.Equal(asset, []byte(filesOfficialTestAtuin)) {
				t.Fatal("installed Atuin theme differs from the official asset")
			}
		})
	}
}

func TestFilesOfficialYaziStylesPreserveLayoutAndInstallSyntaxTheme(t *testing.T) {
	filesOfficialTestClient(t)
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
	if mgr["syntect_theme"] != syntax || !bytes.Equal(filesTestRead(t, syntax), []byte(filesOfficialTestSyntax)) {
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

func TestFilesOfficialYaziManagedStylesPreserveEditedLeaves(t *testing.T) {
	filesOfficialTestClient(t)
	env := filesTestEnv(t)
	path := filesPath("yazi", env)
	port, ok := filesOfficialYazi("catppuccin-mocha")
	if !ok {
		t.Fatal("Catppuccin Yazi source not registered")
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if string(filesTestRead(t, filesYaziManagedStylesPath(env))) != filesOfficialTestYazi {
		t.Fatal("managed styles do not record applied source bytes")
	}
	if err := filesEditTOML(path, func(config map[string]any) error {
		config["mgr"].(map[string]any)["cwd"].(map[string]any)["fg"] = "CUSTOM"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	flavorRoot := filepath.Join(filepath.Dir(path), "flavors", "nord.yazi")
	filesTestWrite(t, filepath.Join(flavorRoot, "flavor.toml"), "[mgr]\ncwd = { fg = 'blue' }\n")
	filesTestWrite(t, filepath.Join(flavorRoot, "tmtheme.xml"), filesOfficialTestSyntax)
	native, ok, err := filesResolveYazi("nord", env)
	if err != nil || !ok || native.Kind != "native" {
		t.Fatalf("native flavor: %#v, %v", native, err)
	}
	if err := filesApplyYazi(native, env); err != nil {
		t.Fatal(err)
	}
	cleaned, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := cleaned["mgr"].(map[string]any)
	if mgr["syntect_theme"] != nil || mgr["cwd"].(map[string]any)["fg"] != "CUSTOM" || mgr["find_keyword"] != nil || mgr["border_symbol"] != nil {
		t.Fatalf("managed style cleanup discarded user edits or left imported values: %#v", mgr)
	}
	flavor := cleaned["flavor"].(map[string]any)
	if flavor["dark"] != "nord" || flavor["light"] != "nord" {
		t.Fatalf("native flavor not selected: %#v", flavor)
	}
}

func TestFilesOfficialYaziFailuresPreserveSettingsAndInstalledSyntax(t *testing.T) {
	for _, failure := range []string{"theme offline", "theme invalid", "syntax offline", "syntax invalid"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath("yazi", env)
			const original = "[mgr]\ncwd = { fg = 'KEEP' }\n"
			filesTestWrite(t, path, original)
			syntaxPath := filepath.Join(filepath.Dir(path), "themeset-catppuccin-mocha.tmTheme")
			filesTestWrite(t, syntaxPath, filesOfficialTestSyntax)
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.String() {
				case filesOfficialTestSource("yazi", "catppuccin-mocha"):
					if failure == "theme offline" {
						return nil, fmt.Errorf("offline")
					}
					body := filesOfficialTestYazi
					if failure == "theme invalid" {
						body = "[mgr]\n"
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				case filesYaziCatppuccinSyntaxSource:
					if failure == "syntax offline" {
						return nil, fmt.Errorf("offline")
					}
					body := filesOfficialTestSyntax
					if failure == "syntax invalid" {
						body = "<html>not a plist</html>"
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				default:
					return nil, fmt.Errorf("unexpected asset request %s", r.URL)
				}
			})
			port, _ := filesOfficialYazi("catppuccin-mocha")
			if err := filesApplyYazi(port, env); err == nil {
				t.Fatal("failed download or native validation accepted")
			}
			if string(filesTestRead(t, path)) != original || string(filesTestRead(t, syntaxPath)) != filesOfficialTestSyntax {
				t.Fatal("failure modified settings or installed syntax")
			}
			if data, err := filesRead(filesYaziManagedStylesPath(env), true); err != nil || data != nil {
				t.Fatalf("failure wrote managed-style snapshot: %q, %v", data, err)
			}
		})
	}
}

func TestFilesOfficialNativeDownloadFailuresLeaveSettingsUntouched(t *testing.T) {
	for _, tc := range []struct {
		id, name, original, invalid string
	}{
		{"lazygit", "dracula", "gui:\n  mouseEvents: false\n", "gui:\n  mouseEvents: true\n"},
		{"lazygit", "catppuccin-mocha", "gui:\n  mouseEvents: false\n", "gui:\n  mouseEvents: true\n"},
		{"lazydocker", "dracula", "gui:\n  scrollHeight: 9\n", "gui:\n  theme: {}\n"},
		{"atuin", "catppuccin-mocha", "enter_accept = false\n", "[theme]\nname = 'wrong-name'\n[colors]\nAlertInfo = '#112233'\n"},
		{"eza", "catppuccin-mocha", "filekinds:\n  directory: {foreground: red}\n", "gui:\n  theme: blue\n"},
	} {
		for _, failure := range []string{"offline", "invalid"} {
			t.Run(tc.id+"/"+tc.name+"/"+failure, func(t *testing.T) {
				env := filesTestEnv(t)
				path := filesPath(tc.id, env)
				filesTestWrite(t, path, tc.original)
				ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
					if r.URL.String() != filesOfficialTestSource(tc.id, tc.name) {
						return nil, fmt.Errorf("unexpected asset request %s", r.URL)
					}
					if failure == "offline" {
						return nil, fmt.Errorf("offline")
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.invalid)), Header: make(http.Header)}, nil
				})
				adapter := filesTestAdapter(t, tc.id)
				port, ok, err := adapter.Resolve(tc.name, env)
				if err != nil || !ok {
					t.Fatalf("official port unavailable: %#v, %v", port, err)
				}
				if err := adapter.Apply(port, env); err == nil {
					t.Fatal("bad official theme accepted")
				}
				if string(filesTestRead(t, path)) != tc.original {
					t.Fatal("failed official theme changed settings")
				}
			})
		}
	}
}

func TestFilesOfficialYaziReapplyFetchesLatestThemeAndSyntax(t *testing.T) {
	env := filesTestEnv(t)
	port, ok := filesOfficialYazi("catppuccin-mocha")
	if !ok {
		t.Fatal("Catppuccin Yazi source not registered")
	}
	theme := filesOfficialTestYazi
	syntax := filesOfficialTestSyntax
	requests := 0
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch r.URL.String() {
		case port.Source:
			body = theme
		case filesYaziCatppuccinSyntaxSource:
			body = syntax
		default:
			return nil, fmt.Errorf("unexpected asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	theme = strings.ReplaceAll(theme, "#94e2d5", "#123456")
	syntax = strings.ReplaceAll(syntax, "<dict/>", "<dict><key>name</key><string>new</string></dict>")
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	path := filesPath("yazi", env)
	if !strings.Contains(string(filesTestRead(t, path)), "#123456") || string(filesTestRead(t, filesYaziManagedStylesPath(env))) != theme || string(filesTestRead(t, filepath.Join(filepath.Dir(path), "themeset-catppuccin-mocha.tmTheme"))) != syntax || requests != 4 {
		t.Fatal("reapply did not fetch latest theme and syntax")
	}
}

func TestFilesOfficialYaziLegacyCleanupUsesCurrentSourceConservatively(t *testing.T) {
	env := filesTestEnv(t)
	path := filesPath("yazi", env)
	syntax := filepath.Join(filepath.Dir(path), "themeset-catppuccin-mocha.tmTheme")
	filesTestWrite(t, path, "[mgr]\nsyntect_theme = '"+syntax+"'\ncwd = { fg = '#94e2d5', bg = 'KEEP' }\nfind_keyword = { fg = 'USER' }\n")
	flavorRoot := filepath.Join(filepath.Dir(path), "flavors", "nord.yazi")
	filesTestWrite(t, filepath.Join(flavorRoot, "flavor.toml"), "[mgr]\ncwd = { fg = 'blue' }\n")
	filesTestWrite(t, filepath.Join(flavorRoot, "tmtheme.xml"), filesOfficialTestSyntax)
	requests := 0
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != filesOfficialTestSource("yazi", "catppuccin-mocha") {
			return nil, fmt.Errorf("unexpected asset request %s", r.URL)
		}
		requests++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(filesOfficialTestYazi)), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveYazi("nord", env)
	if err != nil || !ok {
		t.Fatalf("native flavor unavailable: %#v, %v", port, err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	if requests != 1 || mgr["syntect_theme"] != nil || mgr["cwd"].(map[string]any)["fg"] != nil || mgr["cwd"].(map[string]any)["bg"] != "KEEP" || mgr["find_keyword"].(map[string]any)["fg"] != "USER" {
		t.Fatalf("legacy cleanup discarded user edits or kept matching imported styles: %#v, requests=%d", mgr, requests)
	}
}
