package yazi

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
)

func filesTestYaziFamilyInstall(t *testing.T, name string, env model.Environment) string {
	t.Helper()
	root := filesYaziFlavorRoot(name, env)
	testutil.Write(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#88c0d0' }\n[mode]\nnormal_main = { bg = '#5e81ac' }\n")
	testutil.Write(t, filepath.Join(root, "tmtheme.xml"), "<plist><dict/></plist>")
	return root
}

func TestYaziFamilyNoUnverifiedAcquisition(t *testing.T) {
	env := testutil.Env(t)
	calls := 0
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected acquisition")
	})
	for _, name := range []string{"gruvbox", "nord", "moonfly", "unknown", "dracula", "catppuccin-mocha"} {
		port, ok, err := filesResolveYaziFamily(name, env)
		if err != nil || ok || port != (model.Port{}) {
			t.Fatalf("unverified %s source registered: %#v, %v", name, port, err)
		}
	}
	if calls != 0 {
		t.Fatal("resolution downloaded an unverified asset")
	}
	if _, err := os.Stat(filepath.Join(env.Home, ".cache")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("resolution changed asset cache: %v", err)
	}
}

func TestYaziFamilyCompleteNativeFlavorsSelectAndSwitch(t *testing.T) {
	env := testutil.Env(t)
	env.Paths["yazi"] = filepath.Join(env.Home, "chosen", "theme.toml")
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 2, 3]\ncwd = { italic = true }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	testutil.Write(t, filepath.Join(filepath.Dir(path), "yazi.toml"), "# untouched\n")
	for _, name := range []string{"gruvbox", "nord", "moonfly"} {
		root := filesTestYaziFamilyInstall(t, name, env)
		before := testutil.Read(t, filepath.Join(root, "flavor.toml"))
		port, ok, err := filesResolveYaziFamily(name, env)
		if err != nil || !ok || port.Kind != "native" || port.Source != filesYaziSource || port.File != filepath.Join(root, "flavor.toml") {
			t.Fatalf("complete native %s not recognized: %#v, %v", name, port, err)
		}
		if err := filesApplyYaziFamily(port, env); err != nil {
			t.Fatal(err)
		}
		config, err := shared.TOML(testutil.Read(t, path))
		if err != nil {
			t.Fatal(err)
		}
		flavor := config["flavor"].(map[string]any)
		mgr := config["mgr"].(map[string]any)
		if flavor["dark"] != name || flavor["light"] != name || flavor["use"] != nil || mgr["ratio"] == nil || mgr["cwd"].(map[string]any)["italic"] != true || config["icon"] == nil {
			t.Fatalf("native flavor selection lost settings: %#v", config)
		}
		if !bytes.Equal(before, testutil.Read(t, filepath.Join(root, "flavor.toml"))) || string(testutil.Read(t, filepath.Join(filepath.Dir(path), "yazi.toml"))) != "# untouched\n" {
			t.Fatal("activation modified user flavor or other Yazi files")
		}
		selected := testutil.Read(t, path)
		if err := filesApplyYaziFamily(port, env); err != nil || !bytes.Equal(selected, testutil.Read(t, path)) {
			t.Fatalf("native flavor reapply not idempotent: %v", err)
		}
	}
}

func TestYaziFamilyRemovesOnlyOwnedCatppuccinStyles(t *testing.T) {
	env := testutil.Env(t)
	cat, ok := shared.OfficialPort("yazi", "catppuccin-mocha")
	if !ok {
		t.Fatal("Catppuccin direct style missing")
	}
	if err := filesApplyYazi(cat, env); err != nil {
		t.Fatal(err)
	}
	path := shared.Path("yazi", env)
	if err := shared.EditTOML(path, func(config map[string]any) error {
		mgr := config["mgr"].(map[string]any)
		mgr["cwd"].(map[string]any)["italic"] = true
		mgr["border_style"].(map[string]any)["fg"] = "custom"
		mgr["ratio"] = []int{1, 3, 4}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	root := filesTestYaziFamilyInstall(t, "nord", env)
	port, ok, err := filesResolveYaziFamily("nord", env)
	if err != nil || !ok {
		t.Fatalf("native Nord not found: %v", err)
	}
	if err := filesApplyYaziFamily(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	cwd := mgr["cwd"].(map[string]any)
	if mgr["syntect_theme"] != nil || cwd["fg"] != nil || cwd["italic"] != true || mgr["ratio"] == nil || mgr["border_style"].(map[string]any)["fg"] != "custom" || config["flavor"].(map[string]any)["dark"] != "nord" {
		t.Fatalf("Catppuccin overrides not safely removed: %#v", config)
	}
	if mode, ok := config["mode"].(map[string]any); ok && mode["normal_main"] != nil {
		t.Fatal("old direct mode overrides active Nord")
	}
	if _, err := os.Stat(filepath.Join(root, "tmtheme.xml")); err != nil {
		t.Fatal("installed syntax theme missing")
	}
}

func TestYaziFamilyIncompleteOrMalformedDoesNotActivate(t *testing.T) {
	for _, shape := range []string{"missing syntax", "bad TOML", "old manager", "bad XML", "invalid selection", "removed after resolution"} {
		t.Run(shape, func(t *testing.T) {
			env := testutil.Env(t)
			root := filesTestYaziFamilyInstall(t, "gruvbox", env)
			original := "[flavor]\ndark = 'old'\n[mgr]\nsyntect_theme = '/user/custom.tmTheme'\n"
			testutil.Write(t, shared.Path("yazi", env), original)
			port, _, _ := filesResolveYaziFamily("gruvbox", env)
			switch shape {
			case "missing syntax":
				if err := os.Remove(filepath.Join(root, "tmtheme.xml")); err != nil {
					t.Fatal(err)
				}
			case "bad TOML":
				testutil.Write(t, filepath.Join(root, "flavor.toml"), "broken = [\n")
			case "old manager":
				testutil.Write(t, filepath.Join(root, "flavor.toml"), "[manager]\ncwd = { fg = 'blue' }\n")
			case "bad XML":
				testutil.Write(t, filepath.Join(root, "tmtheme.xml"), "<html/>")
			case "invalid selection":
				testutil.Write(t, shared.Path("yazi", env), "broken = [\n")
				original = "broken = [\n"
			case "removed after resolution":
				if err := os.Remove(filepath.Join(root, "flavor.toml")); err != nil {
					t.Fatal(err)
				}
			}
			_, ok, resolveErr := filesResolveYaziFamily("gruvbox", env)
			switch shape {
			case "missing syntax", "removed after resolution":
				if ok || resolveErr != nil {
					t.Fatalf("missing native asset accepted: %v", resolveErr)
				}
			case "invalid selection":
				if !ok || resolveErr != nil {
					t.Fatalf("complete native asset rejected: %v", resolveErr)
				}
			default:
				if ok || resolveErr == nil {
					t.Fatalf("malformed native asset accepted: %v", resolveErr)
				}
			}
			if err := filesApplyYaziFamily(port, env); err == nil {
				t.Fatal("missing or malformed native asset/config accepted")
			}
			if string(testutil.Read(t, shared.Path("yazi", env))) != original {
				t.Fatal("failed selection modified prior config")
			}
		})
	}
}

func TestYaziFamilyRejectsCustomOrForgedPorts(t *testing.T) {
	env := testutil.Env(t)
	filesTestYaziFamilyInstall(t, "moonfly", env)
	for _, port := range []model.Port{
		{Kind: "custom", Name: "moonfly", File: filepath.Join(filesYaziFlavorRoot("moonfly", env), "flavor.toml")},
		{Kind: "official", Name: "moonfly"},
		{Kind: "native", Name: "gruvbox", File: filepath.Join(filesYaziFlavorRoot("moonfly", env), "flavor.toml")},
	} {
		if err := filesApplyYaziFamily(port, env); err == nil {
			t.Fatalf("non-native or forged Yazi port accepted: %#v", port)
		}
	}
	if _, err := os.Stat(shared.Path("yazi", env)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid port wrote selection: %v", err)
	}
}
