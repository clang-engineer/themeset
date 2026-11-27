package shared_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/clang-engineer/themeset/internal/adapter/atuin"
	"github.com/clang-engineer/themeset/internal/adapter/btop"
	"github.com/clang-engineer/themeset/internal/adapter/eza"
	"github.com/clang-engineer/themeset/internal/adapter/fzf"
	"github.com/clang-engineer/themeset/internal/adapter/lazydocker"
	"github.com/clang-engineer/themeset/internal/adapter/lazygit"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/starship"
	"github.com/clang-engineer/themeset/internal/adapter/yazi"
	"github.com/clang-engineer/themeset/internal/app"
	"github.com/clang-engineer/themeset/internal/model"
)

func filesTestEnv(t *testing.T) model.Environment {
	t.Helper()
	home := t.TempDir()
	return model.Environment{Home: home, ConfigHome: filepath.Join(home, ".config"), OS: "linux", Paths: make(map[string]string)}
}

func TestEditYAMLKeepsCommentsAndUntouchedScalarStyle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	original := "# User settings\ngui:\n  theme:\n    activeBorderColor: ['old'] # Keep this note\n  keybinding:\n    universal: 'keep'\n"
	filesTestWrite(t, path, original)
	if err := shared.EditYAML(path, func(config *yaml.Node) error {
		return shared.MergeYAMLMap(config, map[string]any{"gui": map[string]any{
			"theme": map[string]any{"activeBorderColor": []string{"new"}},
		}})
	}); err != nil {
		t.Fatal(err)
	}
	updated := string(filesTestRead(t, path))
	for _, preserved := range []string{"# User settings", "# Keep this note", "universal: 'keep'"} {
		if !strings.Contains(updated, preserved) {
			t.Fatalf("updated YAML did not preserve %q:\n%s", preserved, updated)
		}
	}
	if strings.Contains(updated, "activeBorderColor: ['old']") || !strings.Contains(updated, "activeBorderColor: [new]") {
		t.Fatalf("updated theme value was not applied:\n%s", updated)
	}
}

func TestEditTOMLPreservesCommentsAndFormatting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# user comment\n\n[ui]\n# keep this note\ntheme = 'old-theme' # inline note\nscale = 1.0\n\n[unrelated]\nvalue = \"unchanged\"\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := shared.EditTOML(path, func(config map[string]any) error {
		ui, err := shared.Table(config, "ui")
		if err != nil {
			return err
		}
		ui["theme"] = "new-theme"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# user comment", "# keep this note", "theme = 'new-theme' # inline note", "scale = 1.0", "[unrelated]", "value = \"unchanged\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("edited TOML lost %q:\n%s", want, got)
		}
	}
}

func TestAtomicBackupAndRollbackRestorePreviousFiles(t *testing.T) {
	root := t.TempDir()
	changed := filepath.Join(root, "config", "existing.toml")
	created := filepath.Join(root, "config", "new.toml")
	if err := os.MkdirAll(filepath.Dir(changed), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, []byte("before\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := shared.Atomic(changed, []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	if err := shared.Atomic(created, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	count, err := shared.Rollback(root)
	if err != nil || count != 2 {
		t.Fatalf("Rollback() = %d, %v", count, err)
	}
	data, err := os.ReadFile(changed)
	if err != nil || string(data) != "before\n" {
		t.Fatalf("existing file = %q, %v", data, err)
	}
	if _, err := os.Stat(created); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("new file remains after rollback: %v", err)
	}
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

func fileTestAdapters() []model.Adapter {
	return []model.Adapter{starship.Adapter(), fzf.Adapter(), yazi.Adapter(), atuin.Adapter(), lazygit.Adapter(), lazydocker.Adapter(), btop.Adapter(), eza.Adapter()}
}

func filesTestAdapter(t *testing.T, id string) model.Adapter {
	t.Helper()
	for _, adapter := range fileTestAdapters() {
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
			path := shared.Path(tt.id, env)
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

func TestFilesNativeSupportRequiresRealInstalledAssets(t *testing.T) {
	env := filesTestEnv(t)
	for _, id := range []string{"yazi", "btop", "eza", "lazygit", "lazydocker", "atuin", "starship", "fzf"} {
		if _, ok, err := filesTestAdapter(t, id).Resolve(model.ThemePort{Native: []string{"not-an-installed-theme-183997"}}, env); ok || err != nil {
			t.Errorf("%s fabricated support: ok=%v err=%v", id, ok, err)
		}
	}
	for _, name := range []string{"dracula", "nord", "gruvbox", "tokyo-night", "atom-one-dark"} {
		if _, ok, err := atuin.Adapter().Resolve(model.ThemePort{Native: []string{name}}, env); ok || err != nil {
			t.Errorf("Atuin must not conflate Harlequin built-ins: %s, %v, %v", name, ok, err)
		}
	}
	rosePine, ok, err := atuin.Adapter().Resolve(app.DefaultPort("rose-pine", "atuin"), env)
	source := app.DefaultPort("rose-pine", "atuin").Official.Source
	if err != nil || !ok || rosePine.Kind != "official" || rosePine.Source != source {
		t.Fatalf("missing registered Rosé Pine Atuin theme: %#v, %v", rosePine, err)
	}
	for _, name := range []string{"default", "autumn", "marine", "pumpkin", "default-powershell", "(none)"} {
		port, ok, err := atuin.Adapter().Resolve(app.DefaultPort(name, "atuin"), env)
		if !ok || err != nil || port.Source == "" {
			t.Fatalf("missing actual Atuin builtin %s: %v", name, err)
		}
	}

	filesTestWrite(t, shared.Path("starship", env), "format = '$directory'\n[palettes.nord]\nblue = '#81a1c1'\n")
	port, ok, err := starship.Adapter().Resolve(app.DefaultPort("nord", "starship"), env)
	if !ok || err != nil {
		t.Fatalf("existing Starship palette unavailable: %v", err)
	}
	if err := starship.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}

	flavor := filepath.Join(filepath.Dir(shared.Path("yazi", env)), "flavors", "nord.yazi")
	filesTestWrite(t, filepath.Join(flavor, "flavor.toml"), "[mgr]\ncwd = { fg = 'blue' }\n")
	if _, ok, err := yazi.Adapter().Resolve(app.DefaultPort("nord", "yazi"), env); ok || err != nil {
		t.Fatalf("incomplete Yazi flavor accepted: %v", err)
	}
	filesTestWrite(t, filepath.Join(flavor, "tmtheme.xml"), "<?xml version=\"1.0\"?><plist/>")
	filesTestWrite(t, shared.Path("yazi", env), "[mgr]\nborder_style = { fg = 'green' }\n")
	port, ok, err = yazi.Adapter().Resolve(app.DefaultPort("nord", "yazi"), env)
	if !ok || err != nil {
		t.Fatalf("installed Yazi flavor unavailable: %v", err)
	}
	if err := yazi.Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	yaziData := filesTestRead(t, shared.Path("yazi", env))
	yaziConfig, err := shared.TOML(yaziData)
	if err != nil {
		t.Fatal(err)
	}
	selected, _ := yaziConfig["flavor"].(map[string]any)
	if selected["dark"] != "nord" || selected["light"] != "nord" || !strings.Contains(string(yaziData), "green") {
		t.Fatalf("Yazi did not preserve overrides/select both modes: %s", yaziData)
	}

	btopTheme := filepath.Join(filepath.Dir(shared.Path("btop", env)), "themes", "gruvbox_dark.theme")
	filesTestWrite(t, btopTheme, "theme[main_fg]=\"#eeeeee\"\n")
	port, ok, err = btop.Adapter().Resolve(app.DefaultPort("gruvbox", "btop"), env)
	if !ok || err != nil || port.Name != "gruvbox_dark" || port.File != btopTheme {
		t.Fatalf("installed btop alias not resolved: %#v, %v", port, err)
	}
}

func TestFilesMalformedConfigsAndUnsafeOptionsDoNotClobber(t *testing.T) {
	for _, id := range []string{"starship", "yazi", "atuin", "lazygit", "lazydocker", "eza"} {
		t.Run(id, func(t *testing.T) {
			env := filesTestEnv(t)
			path := shared.Path(id, env)
			original := "broken = [\n"
			if id == "lazygit" || id == "lazydocker" || id == "eza" {
				original = "broken: [\n"
			}
			filesTestWrite(t, path, original)
			custom := filepath.Join(env.Home, "theme")
			source := "[mgr]\ncwd = { fg = 'red' }\n"
			switch id {
			case "starship":
				source = "palette = 'dracula'\n[palettes.dracula]\nforeground = '#f8f8f2'\n"
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
	env := filesTestEnv(t)
	filesTestWrite(t, shared.Path("fzf", env), "--height=50%\n")
	if err := filesTestAdapter(t, "fzf").Apply(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
		t.Fatal("unrelated fzf options overwritten")
	}
	if string(filesTestRead(t, shared.Path("fzf", env))) != "--height=50%\n" {
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
	if err := eza.Adapter().Apply(model.Port{Kind: "custom", File: custom}, env); err != nil {
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
	if err := shared.Atomic(dangling, []byte("bad")); err == nil {
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
		if actual := shared.Path(id, env); actual != want {
			t.Errorf("%s native path = %q, want %q", id, actual, want)
		}
	}
	legacy := filepath.Join(env.Home, "Library", "Application Support", "jesseduffield", "lazydocker", "config.yml")
	filesTestWrite(t, legacy, "")
	if actual := shared.Path("lazydocker", env); actual != legacy {
		t.Errorf("legacy lazydocker path ignored: %s", actual)
	}
	env.ConfigHomeExplicit = true
	if actual := shared.Path("lazydocker", env); actual != filepath.Join(env.ConfigHome, "lazydocker", "config.yml") {
		t.Errorf("explicit XDG ignored: %s", actual)
	}
	for _, adapter := range fileTestAdapters() {
		env.Paths[adapter.ID] = "~/override/" + adapter.ID
		if actual := shared.Path(adapter.ID, env); actual != filepath.Join(env.Home, "override", adapter.ID) {
			t.Errorf("%s path override ignored: %s", adapter.ID, actual)
		}
	}
}

func TestFilesAtuinExistingLegacyAndModernPaths(t *testing.T) {
	env := filesTestEnv(t)
	modern := filepath.Join(env.Home, ".atuin", "config.toml")
	legacy := filepath.Join(env.ConfigHome, "atuin", "config.toml")
	if actual := shared.Path("atuin", env); actual != modern {
		t.Errorf("first-install Atuin path = %s", actual)
	}
	filesTestWrite(t, legacy, "enter_accept = false\n")
	if actual := shared.Path("atuin", env); actual != legacy {
		t.Errorf("existing legacy Atuin path ignored: %s", actual)
	}
	filesTestWrite(t, modern, "enter_accept = true\n")
	if actual := shared.Path("atuin", env); actual != modern {
		t.Errorf("modern Atuin precedence ignored: %s", actual)
	}
}
