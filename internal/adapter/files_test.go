package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesTestEnv(t *testing.T) model.Environment {
	t.Helper()
	home := t.TempDir()
	return model.Environment{Home: home, ConfigHome: filepath.Join(home, ".config"), OS: "linux", Paths: make(map[string]string)}
}

func filesTestWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func filesTestRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func filesTestAdapter(t *testing.T, id string) model.Adapter {
	t.Helper()
	for _, adapter := range fileAdapters() {
		if adapter.ID == id {
			return adapter
		}
	}
	t.Fatalf("adapter %q missing", id)
	return model.Adapter{}
}

func TestFilesCustomPortsPreserveNonThemeConfiguration(t *testing.T) {
	tests := []struct {
		id, existing, custom string
		want, reject         []string
	}{
		{
			id: "starship", existing: "format = '$directory'\nadd_newline = false\n[palettes.keep]\nred = '#111111'\n",
			custom: "palette = 'sample'\nformat = 'DO NOT IMPORT'\n[palettes.sample]\nred = '#ff0000'\n",
			want:   []string{"$directory", "add_newline = false", "sample", "#ff0000", "#111111"}, reject: []string{"DO NOT IMPORT"},
		},
		{
			id: "fzf", existing: "--color=fg:#111111\n",
			custom: "# an app-native fzf options theme\n--color=fg:#ff0000\n\n",
			want:   []string{"--color=fg:#ff0000"}, reject: []string{"# an app-native", "#111111"},
		},
		{
			id: "yazi", existing: "[mgr]\nborder_style = { fg = 'blue' }\n[status]\nseparator_open = 'x'\n",
			custom: "[mgr]\ncwd = { fg = 'red' }\n", want: []string{"border_style", "blue", "separator_open", "cwd", "red"},
		},
		{
			id: "atuin", existing: "enter_accept = false\nsearch_mode = 'prefix'\n[theme]\ndebug = true\n",
			custom: "[theme]\nname = 'sample'\nparent = 'autumn'\n[colors]\nAlertInfo = 'red'\n",
			want:   []string{"enter_accept = false", "prefix", "debug = true", "sample"}, reject: []string{"AlertInfo"},
		},
		{
			id: "lazygit", existing: "gui:\n  mouseEvents: false\n  theme:\n    inactiveBorderColor: [blue]\ngit:\n  autoFetch: false\n",
			custom: "gui:\n  mouseEvents: true\n  theme:\n    activeBorderColor: [red, bold]\ngit:\n  autoFetch: true\n",
			want:   []string{"mouseEvents: false", "autoFetch: false", "inactiveBorderColor", "blue", "activeBorderColor", "red"},
		},
		{
			id: "lazydocker", existing: "gui:\n  scrollHeight: 9\n  theme:\n    optionsTextColor: [blue]\nlogs:\n  tail: '200'\n",
			custom: "gui:\n  scrollHeight: 100\n  theme:\n    activeBorderColor: [red]\nlogs:\n  tail: '1'\n",
			want:   []string{"scrollHeight: 9", "200", "optionsTextColor", "blue", "activeBorderColor", "red"}, reject: []string{"scrollHeight: 100"},
		},
		{
			id: "btop", existing: "# keep this comment\ncolor_theme = \"Default\"\nupdate_ms = 1234\n",
			custom: "# native asset\ntheme[main_fg]=\"#ff0000\"\n",
			want:   []string{"# keep this comment", "update_ms = 1234", "color_theme = \"themeset\""}, reject: []string{"#ff0000"},
		},
		{
			id: "eza", existing: "filenames:\n  Makefile:\n    icon:\n      glyph: X\nfilekinds:\n  directory:\n    is_bold: true\n",
			custom: "filekinds:\n  directory:\n    foreground: Red\n",
			want:   []string{"Makefile", "glyph: X", "is_bold: true", "foreground: Red"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath(tt.id, env)
			filesTestWrite(t, path, tt.existing)
			custom := filepath.Join(env.Home, "registered", tt.id)
			filesTestWrite(t, custom, tt.custom)
			if err := filesTestAdapter(t, tt.id).Apply(model.Port{Kind: "custom", File: custom}, env); err != nil {
				t.Fatal(err)
			}
			actual := string(filesTestRead(t, path))
			for _, want := range tt.want {
				if !strings.Contains(actual, want) {
					t.Errorf("missing %q in %s", want, actual)
				}
			}
			for _, reject := range tt.reject {
				if strings.Contains(actual, reject) {
					t.Errorf("unexpected %q in %s", reject, actual)
				}
			}
		})
	}
}

func TestFilesOfficialMoonflyPreservesApplicationConfiguration(t *testing.T) {
	t.Run("starship", func(t *testing.T) {
		env := filesTestEnv(t)
		path := filesPath("starship", env)
		filesTestWrite(t, path, "format = '$directory'\nadd_newline = false\n[directory]\ntruncation_length = 2\n[character]\nsuccess_symbol = 'KEEP'\n[palettes.keep]\nred = '#111111'\n")
		port, ok, err := filesResolveStarship("moonfly", env)
		if err != nil || !ok || port.Kind != "official" || port.Source != filesMoonflySource+"moonfly-starship.toml" {
			t.Fatalf("missing pinned official Moonfly: %#v, %v", port, err)
		}
		if err := filesApplyStarship(port, env); err != nil {
			t.Fatal(err)
		}
		config, err := filesTOML(filesTestRead(t, path))
		if err != nil {
			t.Fatal(err)
		}
		directory := config["directory"].(map[string]any)
		character := config["character"].(map[string]any)
		palettes := config["palettes"].(map[string]any)
		palette := palettes["moonfly"].(map[string]any)
		if config["palette"] != "moonfly" || config["format"] != "$directory" || config["add_newline"] != false || directory["truncation_length"] != int64(2) || character["success_symbol"] != "KEEP" || palettes["keep"] == nil {
			t.Fatalf("unrelated Starship settings changed: %#v", config)
		}
		want := map[string]string{"fern": "#87d787", "lavender": "#adadf3", "orange": "#f09479", "red": "#f07496"}
		if len(palette) != len(want) {
			t.Fatalf("official palette contains fabricated colors: %#v", palette)
		}
		for key, value := range want {
			if palette[key] != value {
				t.Errorf("Moonfly %s = %v, want %s", key, palette[key], value)
			}
		}
	})
	t.Run("fzf", func(t *testing.T) {
		env := filesTestEnv(t)
		unrelated := filepath.Join(env.Home, ".fzf.zsh")
		original := "export FZF_DEFAULT_OPTS='--height=40% --reverse'\n"
		filesTestWrite(t, unrelated, original)
		port, ok, err := filesResolveFZF("moonfly", env)
		if err != nil || !ok || port.Kind != "official" || port.Source != filesMoonflySource+"moonfly-fzf.sh" {
			t.Fatalf("missing pinned official Moonfly: %#v, %v", port, err)
		}
		if err := filesApplyFZF(port, env); err != nil {
			t.Fatal(err)
		}
		first := string(filesTestRead(t, filesPath("fzf", env)))
		if err := filesFZFOptions([]byte(first)); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(first, "--color=bg:#080808\n") || !strings.Contains(first, "--color=pointer:#ff5189\n") || len(strings.Fields(first)) != 14 {
			t.Fatalf("official Moonfly options missing: %s", first)
		}
		if err := filesApplyFZF(port, env); err != nil {
			t.Fatal(err)
		}
		if string(filesTestRead(t, filesPath("fzf", env))) != first || string(filesTestRead(t, unrelated)) != original {
			t.Fatal("fzf reapply changed options or unrelated shell configuration")
		}
	})
}

func TestFilesNativeSupportRequiresRealInstalledAssets(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"yazi", "btop", "eza", "lazygit", "lazydocker", "atuin", "starship", "fzf"} {
		if _, ok, err := filesTestAdapter(t, id).Resolve("not-an-installed-theme-183997", env); ok || err != nil {
			t.Errorf("%s fabricated support: ok=%v err=%v", id, ok, err)
		}
	}
	for _, name := range []string{"dracula", "nord", "gruvbox", "tokyo-night", "atom-one-dark"} {
		if _, ok, err := filesResolveAtuin(name, env); ok || err != nil {
			t.Errorf("Atuin must not conflate Harlequin built-ins: %s, %v, %v", name, ok, err)
		}
	}
	rosePine, ok, err := filesResolveAtuin("rose-pine", env)
	recipe, _ := rosePineRecipe("atuin")
	if err != nil || !ok || rosePine.Kind != "official" || rosePine.Source != recipe.url {
		t.Fatalf("missing registered Rosé Pine Atuin theme: %#v, %v", rosePine, err)
	}
	for _, name := range []string{"default", "autumn", "marine", "pumpkin", "default-powershell", "(none)"} {
		port, ok, err := filesResolveAtuin(name, env)
		if !ok || err != nil || port.Source == "" {
			t.Fatalf("missing actual Atuin builtin %s: %v", name, err)
		}
	}

	filesTestWrite(t, filesPath("starship", env), "format = '$directory'\n[palettes.nord]\nblue = '#81a1c1'\n")
	port, ok, err := filesResolveStarship("nord", env)
	if !ok || err != nil {
		t.Fatalf("existing Starship palette unavailable: %v", err)
	}
	if err := filesApplyStarship(port, env); err != nil {
		t.Fatal(err)
	}

	flavor := filepath.Join(filepath.Dir(filesPath("yazi", env)), "flavors", "nord.yazi")
	filesTestWrite(t, filepath.Join(flavor, "flavor.toml"), "[mgr]\ncwd = { fg = 'blue' }\n")
	if _, ok, err := filesResolveYazi("nord", env); ok || err != nil {
		t.Fatalf("incomplete Yazi flavor accepted: %v", err)
	}
	filesTestWrite(t, filepath.Join(flavor, "tmtheme.xml"), "<?xml version=\"1.0\"?><plist/>")
	filesTestWrite(t, filesPath("yazi", env), "[mgr]\nborder_style = { fg = 'green' }\n")
	port, ok, err = filesResolveYazi("nord", env)
	if !ok || err != nil {
		t.Fatalf("installed Yazi flavor unavailable: %v", err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	yaziData := filesTestRead(t, filesPath("yazi", env))
	yaziConfig, err := filesTOML(yaziData)
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := yaziConfig["flavor"].(map[string]any)
	if selected["dark"] != "nord" || selected["light"] != "nord" || !strings.Contains(string(yaziData), "green") {
		t.Fatalf("Yazi did not preserve overrides/select both modes: %s", yaziData)
	}

	btopTheme := filepath.Join(filepath.Dir(filesPath("btop", env)), "themes", "gruvbox_dark.theme")
	filesTestWrite(t, btopTheme, "theme[main_fg]=\"#eeeeee\"\n")
	port, ok, err = filesResolveBtop("gruvbox", env)
	if !ok || err != nil || port.Name != "gruvbox_dark" || port.File != btopTheme {
		t.Fatalf("installed btop alias not resolved: %#v, %v", port, err)
	}
}

func TestFilesMalformedConfigsAndUnsafeOptionsDoNotClobber(t *testing.T) {
	for _, id := range []string{"starship", "yazi", "atuin", "lazygit", "lazydocker", "eza"} {
		t.Run(id, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath(id, env)
			original := "broken = [\n"
			if id == "lazygit" || id == "lazydocker" || id == "eza" {
				original = "broken: [\n"
			}
			filesTestWrite(t, path, original)
			custom := filepath.Join(env.Home, "theme")
			source := "[mgr]\ncwd = { fg = 'red' }\n"
			switch id {
			case "starship":
				source = string(filesStarshipDracula)
			case "atuin":
				source = "[theme]\nname = 'sample'\n[colors]\nAlertInfo = 'red'\n"
			case "lazygit", "lazydocker":
				source = "gui:\n  theme:\n    activeBorderColor: [red]\n"
			case "eza":
				source = "filekinds:\n  directory:\n    foreground: Red\n"
			}
			filesTestWrite(t, custom, source)
			if err := filesTestAdapter(t, id).Apply(model.Port{Kind: "custom", File: custom}, env); err == nil {
				t.Fatal("malformed configuration accepted")
			}
			if string(filesTestRead(t, path)) != original {
				t.Fatal("malformed configuration overwritten")
			}
		})
	}
	for _, options := range []string{"--bind=enter:execute(rm -rf /)\n", "--color=fg:red --preview=bad\n", "$(touch /tmp/bad)\n", "--color='fg:red'\n"} {
		if err := filesFZFOptions([]byte(options)); err == nil {
			t.Errorf("unsafe/non-theme fzf options accepted: %q", options)
		}
	}
	env := filesTestEnv(t)
	filesTestWrite(t, filesPath("fzf", env), "--height=50%\n")
	if err := filesApplyFZF(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
		t.Fatal("unrelated fzf options overwritten")
	}
	if string(filesTestRead(t, filesPath("fzf", env))) != "--height=50%\n" {
		t.Fatal("unrelated fzf options changed")
	}
}

func TestFilesAtomicPreservesSymlinkAndMode(t *testing.T) {
	env := filesTestEnv(t)
	target := filepath.Join(env.Home, "managed", "config.yml")
	filesTestWrite(t, target, "filekinds:\n  directory:\n    is_bold: true\n")
	if err := os.Chmod(target, 0640); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(env.Home, "link.yml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	env.Paths["eza"] = "~/link.yml"
	custom := filepath.Join(env.Home, "native-theme.yml")
	filesTestWrite(t, custom, "filekinds:\n  directory:\n    foreground: Red\n")
	if err := filesApplyEza(model.Port{Kind: "custom", File: custom}, env); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink clobbered: %v", err)
	}
	info, err = os.Stat(target)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("target mode changed: %v", err)
	}
	if !strings.Contains(string(filesTestRead(t, target)), "is_bold: true") {
		t.Fatal("symlink target lost unrelated style")
	}
	dangling := filepath.Join(env.Home, "dangling")
	if err := os.Symlink(filepath.Join(env.Home, "missing"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := filesAtomic(dangling, []byte("bad")); err == nil {
		t.Fatal("dangling symlink accepted")
	}
}

func TestFilesPlatformPathsAndOverrides(t *testing.T) {
	env := filesTestEnv(t)
	env.OS = "darwin"
	for _, id := range []string{"lazygit", "lazydocker", "eza"} {
		base := "config.yml"
		if id == "eza" {
			base = "theme.yml"
		}
		want := filepath.Join(env.Home, "Library", "Application Support", id, base)
		if actual := filesPath(id, env); actual != want {
			t.Errorf("%s native path = %q, want %q", id, actual, want)
		}
	}
	legacy := filepath.Join(env.Home, "Library", "Application Support", "jesseduffield", "lazydocker", "config.yml")
	filesTestWrite(t, legacy, "")
	if actual := filesPath("lazydocker", env); actual != legacy {
		t.Errorf("legacy lazydocker path ignored: %s", actual)
	}
	env.ConfigHomeExplicit = true
	if actual := filesPath("lazydocker", env); actual != filepath.Join(env.ConfigHome, "lazydocker", "config.yml") {
		t.Errorf("explicit XDG ignored: %s", actual)
	}
	for _, adapter := range fileAdapters() {
		env.Paths[adapter.ID] = "~/override/" + adapter.ID
		if actual := filesPath(adapter.ID, env); actual != filepath.Join(env.Home, "override", adapter.ID) {
			t.Errorf("%s path override ignored: %s", adapter.ID, actual)
		}
	}
}

func TestFilesAtuinExistingLegacyAndModernPaths(t *testing.T) {
	env := filesTestEnv(t)
	modern := filepath.Join(env.Home, ".atuin", "config.toml")
	legacy := filepath.Join(env.ConfigHome, "atuin", "config.toml")
	if actual := filesPath("atuin", env); actual != modern {
		t.Errorf("first-install Atuin path = %s", actual)
	}
	filesTestWrite(t, legacy, "enter_accept = false\n")
	if actual := filesPath("atuin", env); actual != legacy {
		t.Errorf("existing legacy Atuin path ignored: %s", actual)
	}
	filesTestWrite(t, modern, "enter_accept = true\n")
	if actual := filesPath("atuin", env); actual != modern {
		t.Errorf("modern Atuin precedence ignored: %s", actual)
	}
}
