package adapter

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesTestYaziFamilyInstall(t *testing.T, name string, env model.Environment) string {
	t.Helper()
	root := filesYaziFlavorRoot(name, env)
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#88c0d0' }\n[mode]\nnormal_main = { bg = '#5e81ac' }\n")
	filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), "<plist><dict/></plist>")
	return root
}

func TestYaziFamilyNoUnverifiedAcquisition(t *testing.T) {
	env := filesTestEnv(t)
	calls := 0
	filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
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
	env := filesTestEnv(t)
	env.Paths["yazi"] = filepath.Join(env.Home, "chosen", "theme.toml")
	path := filesPath("yazi", env)
	filesTestWrite(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 2, 3]\ncwd = { italic = true }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	filesTestWrite(t, filepath.Join(filepath.Dir(path), "yazi.toml"), "# untouched\n")
	for _, name := range []string{"gruvbox", "nord", "moonfly"} {
		root := filesTestYaziFamilyInstall(t, name, env)
		before := filesTestRead(t, filepath.Join(root, "flavor.toml"))
		port, ok, err := filesResolveYaziFamily(name, env)
		if err != nil || !ok || port.Kind != "native" || port.Source != filesYaziSource || port.File != filepath.Join(root, "flavor.toml") {
			t.Fatalf("complete native %s not recognized: %#v, %v", name, port, err)
		}
		if err := filesApplyYaziFamily(port, env); err != nil {
			t.Fatal(err)
		}
		config, err := filesTOML(filesTestRead(t, path))
		if err != nil {
			t.Fatal(err)
		}
		flavor := config["flavor"].(map[string]any)
		mgr := config["mgr"].(map[string]any)
		if flavor["dark"] != name || flavor["light"] != name || flavor["use"] != nil || mgr["ratio"] == nil || mgr["cwd"].(map[string]any)["italic"] != true || config["icon"] == nil {
			t.Fatalf("native flavor selection lost settings: %#v", config)
		}
		if !bytes.Equal(before, filesTestRead(t, filepath.Join(root, "flavor.toml"))) || string(filesTestRead(t, filepath.Join(filepath.Dir(path), "yazi.toml"))) != "# untouched\n" {
			t.Fatal("activation modified user flavor or other Yazi files")
		}
		selected := filesTestRead(t, path)
		if err := filesApplyYaziFamily(port, env); err != nil || !bytes.Equal(selected, filesTestRead(t, path)) {
			t.Fatalf("native flavor reapply not idempotent: %v", err)
		}
	}
}

func TestYaziFamilyRemovesOnlyOwnedCatppuccinStyles(t *testing.T) {
	env := filesTestEnv(t)
	cat, ok := filesOfficialYazi("catppuccin-mocha")
	if !ok {
		t.Fatal("Catppuccin direct style missing")
	}
	if err := filesApplyYazi(cat, env); err != nil {
		t.Fatal(err)
	}
	path := filesPath("yazi", env)
	if err := filesEditTOML(path, func(config map[string]any) error {
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
	config, err := filesTOML(filesTestRead(t, path))
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
			env := filesTestEnv(t)
			root := filesTestYaziFamilyInstall(t, "gruvbox", env)
			original := "[flavor]\ndark = 'old'\n[mgr]\nsyntect_theme = '/user/custom.tmTheme'\n"
			filesTestWrite(t, filesPath("yazi", env), original)
			port, _, _ := filesResolveYaziFamily("gruvbox", env)
			switch shape {
			case "missing syntax":
				if err := os.Remove(filepath.Join(root, "tmtheme.xml")); err != nil {
					t.Fatal(err)
				}
			case "bad TOML":
				filesTestWrite(t, filepath.Join(root, "flavor.toml"), "broken = [\n")
			case "old manager":
				filesTestWrite(t, filepath.Join(root, "flavor.toml"), "[manager]\ncwd = { fg = 'blue' }\n")
			case "bad XML":
				filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), "<html/>")
			case "invalid selection":
				filesTestWrite(t, filesPath("yazi", env), "broken = [\n")
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
			if string(filesTestRead(t, filesPath("yazi", env))) != original {
				t.Fatal("failed selection modified prior config")
			}
		})
	}
}

func TestYaziFamilyRejectsCustomOrForgedPorts(t *testing.T) {
	env := filesTestEnv(t)
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
	if _, err := os.Stat(filesPath("yazi", env)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid port wrote selection: %v", err)
	}
}
