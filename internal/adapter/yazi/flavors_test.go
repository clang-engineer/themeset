package yazi

import (
	"bytes"
	"errors"
	"fmt"
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

func filesTestResolveYazi(name string, env model.Environment) (model.Port, bool, error) {
	return filesResolveYazi(catalog.Port(name, "yazi"), env)
}

func filesTestYaziDraculaFixture(t *testing.T, name string) []byte {
	t.Helper()
	return testutil.Read(t, filepath.Join("..", "testdata", "yazi-dracula", name))
}

func filesTestYaziDraculaResponse(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	if !strings.HasPrefix(req.URL.String(), shared.YaziDraculaRaw) {
		t.Fatalf("unexpected asset source: %s", req.URL)
	}
	name := strings.TrimPrefix(req.URL.String(), shared.YaziDraculaRaw)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(filesTestYaziDraculaFixture(t, name))), Header: make(http.Header)}
}

func TestYaziDraculaResolveDoesNotDownload(t *testing.T) {
	env := testutil.Env(t)
	calls := 0
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network must not be used by resolve")
	})
	port, ok, err := filesTestResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "official" || port.File != "" || port.Source != shared.YaziDraculaSource {
		t.Fatalf("missing registered official flavor: %#v, %v", port, err)
	}
	if calls != 0 {
		t.Fatal("resolver performed acquisition")
	}
	for _, path := range []string{filesYaziDraculaRoot(env), filepath.Join(env.Home, ".cache"), shared.Path("yazi", env)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("resolver wrote %s: %v", path, err)
		}
	}
}

func TestYaziDraculaInvalidSelectionConfigDoesNotAcquire(t *testing.T) {
	for _, original := range []string{"broken = [\n", "flavor = 'not a table'\n"} {
		env := testutil.Env(t)
		testutil.Write(t, shared.Path("yazi", env), original)
		testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
			t.Fatal("invalid destination config triggered acquisition")
			return nil, errors.New("offline")
		})
		if err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
			t.Fatal("invalid selection configuration accepted")
		}
		if string(testutil.Read(t, shared.Path("yazi", env))) != original {
			t.Fatal("invalid selection configuration changed")
		}
		if _, err := os.Stat(filesYaziDraculaRoot(env)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid selection configuration installed a flavor")
		}
	}
}

func TestYaziDraculaAcquiresCompleteFlavorAndActivates(t *testing.T) {
	env := testutil.Env(t)
	env.Paths["yazi"] = filepath.Join(env.Home, "explicit", "theme.toml")
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 3, 4]\nborder_style = { fg = 'custom' }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		testutil.Write(t, filepath.Join(filepath.Dir(path), name), "# untouched "+name+"\n")
	}
	calls := make(map[string]int)
	testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
		calls[filepath.Base(req.URL.Path)]++
		return filesTestYaziDraculaResponse(t, req), nil
	})
	port, _, err := filesTestResolveYazi("dracula", env)
	if err != nil {
		t.Fatal(err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	root := filesYaziDraculaRoot(env)
	for _, asset := range shared.YaziDraculaAssets {
		data := testutil.Read(t, filepath.Join(root, asset))
		if !bytes.Equal(data, filesTestYaziDraculaFixture(t, asset)) || calls[asset] != 1 {
			t.Fatalf("asset %s not acquired exactly once from the current branch", asset)
		}
	}
	if string(testutil.Read(t, filepath.Join(root, filesYaziSourceMarker))) != shared.YaziDraculaSource+"\n" {
		t.Fatal("official flavor provenance was not recorded")
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	flavor := config["flavor"].(map[string]any)
	mgr := config["mgr"].(map[string]any)
	if flavor["dark"] != "dracula" || flavor["light"] != "dracula" || flavor["use"] != nil || mgr["ratio"] == nil || mgr["border_style"].(map[string]any)["fg"] != "custom" || config["icon"] == nil {
		t.Fatalf("selection or user configuration changed: %#v", config)
	}
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		if string(testutil.Read(t, filepath.Join(filepath.Dir(path), name))) != "# untouched "+name+"\n" {
			t.Fatalf("changed %s", name)
		}
	}
	first := testutil.Read(t, path)
	port, ok, err := filesTestResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("installed official flavor not recognized: %#v, %v", port, err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, testutil.Read(t, path)) {
		t.Fatal("reapply changed configuration")
	}
	for name, count := range calls {
		if count != 1 {
			t.Fatalf("reapply downloaded %s again", name)
		}
	}
}

func TestYaziDraculaReusesCompatibleInstalledFlavorOffline(t *testing.T) {
	env := testutil.Env(t)
	root := filesYaziDraculaRoot(env)
	testutil.Write(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#123456' }\n")
	testutil.Write(t, filepath.Join(root, "tmtheme.xml"), string(filesTestYaziDraculaFixture(t, "tmtheme.xml")))
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("complete native flavor triggered acquisition")
		return nil, errors.New("offline")
	})
	port, ok, err := filesTestResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") || port.Source != shared.YaziDraculaSource {
		t.Fatalf("compatible native flavor not recognized: %#v, %v", port, err)
	}
	before := testutil.Read(t, port.File)
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, testutil.Read(t, port.File)) {
		t.Fatal("native flavor overwritten")
	}
}

func TestYaziDraculaTreatsUnmarkedOfficialBytesAsNative(t *testing.T) {
	env := testutil.Env(t)
	root := filesYaziDraculaRoot(env)
	for _, asset := range shared.YaziDraculaAssets {
		testutil.Write(t, filepath.Join(root, asset), string(filesTestYaziDraculaFixture(t, asset)))
	}
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("complete independently installed flavor triggered acquisition")
		return nil, errors.New("offline")
	})
	port, ok, err := filesTestResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") {
		t.Fatalf("unmarked independent flavor was not native: %#v, %v", port, err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filesYaziSourceMarker)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("independent flavor was marked as official")
	}
}

func TestYaziDraculaFailurePreservesSelectionAndIncompleteFlavor(t *testing.T) {
	for _, failure := range []string{"network", "status", "malformed syntax"} {
		t.Run(failure, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("yazi", env)
			original := "[flavor]\ndark = 'keep'\nlight = 'keep'\n[mgr]\ncwd = { fg = 'custom' }\n"
			testutil.Write(t, path, original)
			root := filesYaziDraculaRoot(env)
			testutil.Write(t, filepath.Join(root, "flavor.toml"), string(filesTestYaziDraculaFixture(t, "flavor.toml")))
			testutil.Write(t, filepath.Join(root, "keep.txt"), "user file")
			testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
				switch failure {
				case "network":
					if filepath.Base(req.URL.Path) == "LICENSE" {
						return nil, errors.New("offline")
					}
				case "status":
					if filepath.Base(req.URL.Path) == "LICENSE" {
						return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("missing"))}, nil
					}
				case "malformed syntax":
					if filepath.Base(req.URL.Path) == "tmtheme.xml" {
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("<html>not a tmtheme</html>")), Header: make(http.Header)}, nil
					}
				}
				return filesTestYaziDraculaResponse(t, req), nil
			})
			port, _, err := filesTestResolveYazi("dracula", env)
			if err != nil {
				t.Fatal(err)
			}
			if err := filesApplyYazi(port, env); err == nil {
				t.Fatal("failed acquisition accepted")
			}
			if string(testutil.Read(t, path)) != original || string(testutil.Read(t, filepath.Join(root, "keep.txt"))) != "user file" {
				t.Fatal("failed acquisition changed prior selection or user files")
			}
			if _, err := os.Stat(filepath.Join(root, "tmtheme.xml")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed acquisition exposed a partial complete flavor")
			}
		})
	}
}

func TestYaziDraculaCompletesOnlyMatchingPartialAssets(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "matching", true: "conflicting"}[conflict], func(t *testing.T) {
			env := testutil.Env(t)
			root := filesYaziDraculaRoot(env)
			partial := string(filesTestYaziDraculaFixture(t, "flavor.toml"))
			if conflict {
				partial = "[mgr]\ncwd = { fg = 'keep' }\n"
			}
			testutil.Write(t, filepath.Join(root, "flavor.toml"), partial)
			testutil.Write(t, filepath.Join(root, "keep.txt"), "user file")
			testutil.Write(t, shared.Path("yazi", env), "[flavor]\ndark = 'keep'\n")
			before := testutil.Read(t, shared.Path("yazi", env))
			calls := 0
			testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
				calls++
				return filesTestYaziDraculaResponse(t, req), nil
			})
			err := filesApplyYazi(*catalog.Port("dracula", "yazi").Official, env)
			if conflict {
				if err == nil || calls != 1 || !bytes.Equal(before, testutil.Read(t, shared.Path("yazi", env))) {
					t.Fatal("conflicting native partial flavor was not preserved")
				}
				if _, err := os.Stat(filepath.Join(root, filesYaziSourceMarker)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("conflicting partial flavor was marked official")
				}
			} else if err != nil || calls != len(shared.YaziDraculaAssets) {
				t.Fatalf("matching partial flavor not completed: calls=%d, %v", calls, err)
			}
			if string(testutil.Read(t, filepath.Join(root, "keep.txt"))) != "user file" || string(testutil.Read(t, filepath.Join(root, "flavor.toml"))) != partial {
				t.Fatal("existing partial flavor data changed")
			}
		})
	}
}

func TestYaziDraculaRejectsMalformedDownloadsWithoutChangingSelection(t *testing.T) {
	for _, failure := range []string{"invalid TOML", "missing manager", "invalid syntax", "empty license"} {
		t.Run(failure, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("yazi", env)
			original := "[flavor]\ndark = 'keep'\n"
			testutil.Write(t, path, original)
			testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
				name := filepath.Base(req.URL.Path)
				var invalid string
				switch failure {
				case "invalid TOML":
					if name == "flavor.toml" {
						invalid = "broken = ["
					}
				case "missing manager":
					if name == "flavor.toml" {
						invalid = "[manager]\ncwd = 'old'\n"
					}
				case "invalid syntax":
					if name == "tmtheme.xml" {
						invalid = "<html>not a tmtheme</html>"
					}
				case "empty license":
					if name == "LICENSE" {
						return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
					}
				}
				if invalid != "" {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(invalid)), Header: make(http.Header)}, nil
				}
				return filesTestYaziDraculaResponse(t, req), nil
			})
			if err := filesApplyYazi(*catalog.Port("dracula", "yazi").Official, env); err == nil {
				t.Fatal("invalid upstream flavor was installed")
			}
			if string(testutil.Read(t, path)) != original {
				t.Fatal("invalid upstream flavor changed selection")
			}
			if _, err := os.Stat(filesYaziDraculaRoot(env)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid upstream flavor created the destination directory")
			}
		})
	}
}

func TestYaziDraculaRejectsMalformedInstalledSourceShape(t *testing.T) {
	for _, shape := range []string{"invalid TOML", "old manager", "invalid syntax"} {
		t.Run(shape, func(t *testing.T) {
			env := testutil.Env(t)
			root := filesYaziDraculaRoot(env)
			flavor, syntax := "[mgr]\ncwd = { fg = 'blue' }\n", "<plist><dict/></plist>"
			switch shape {
			case "invalid TOML":
				flavor = "broken = ["
			case "old manager":
				flavor = "[manager]\ncwd = { fg = 'blue' }\n"
			case "invalid syntax":
				syntax = "<html>not a tmtheme</html>"
			}
			testutil.Write(t, filepath.Join(root, "flavor.toml"), flavor)
			testutil.Write(t, filepath.Join(root, "tmtheme.xml"), syntax)
			original := "[flavor]\ndark = 'keep'\n"
			testutil.Write(t, shared.Path("yazi", env), original)
			testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
				t.Fatal("malformed native flavor triggered acquisition")
				return nil, errors.New("offline")
			})
			if _, _, err := filesTestResolveYazi("dracula", env); err == nil {
				t.Fatal("malformed installed source accepted by resolver")
			}
			if err := filesApplyYazi(*catalog.Port("dracula", "yazi").Official, env); err == nil {
				t.Fatal("malformed installed source accepted by apply")
			}
			if string(testutil.Read(t, shared.Path("yazi", env))) != original {
				t.Fatal("malformed source changed selection")
			}
		})
	}
}

func TestYaziDraculaRemovesOnlyOwnedPreviousCatppuccinLeaves(t *testing.T) {
	env := testutil.Env(t)
	cat := *catalog.Port("catppuccin-mocha", "yazi").Official
	if err := filesApplyYazi(cat, env); err != nil {
		t.Fatal(err)
	}
	path := shared.Path("yazi", env)
	if err := shared.EditTOML(path, func(config map[string]any) error {
		mgr := config["mgr"].(map[string]any)
		mgr["cwd"].(map[string]any)["italic"] = true
		mgr["border_style"].(map[string]any)["fg"] = "user-custom"
		mgr["ratio"] = []int{1, 3, 4}
		config["icon"] = map[string]any{"prepend_dirs": []map[string]any{{"name": "work", "text": "W"}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
		return filesTestYaziDraculaResponse(t, req), nil
	})
	if err := filesApplyYazi(*catalog.Port("dracula", "yazi").Official, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	cwd := mgr["cwd"].(map[string]any)
	if mgr["syntect_theme"] != nil || cwd["fg"] != nil || cwd["italic"] != true || mgr["border_style"].(map[string]any)["fg"] != "user-custom" || mgr["ratio"] == nil || config["icon"] == nil {
		t.Fatalf("owned styles leaked or user styles were removed: %#v", config)
	}
	// Prove actual inherited Dracula colors, not merely selector fields: the
	// installed flavor supplies the upstream cyan cwd/purple normal mode and
	// no remaining Catppuccin foreground can override either in theme.toml.
	installed, err := shared.TOML(testutil.Read(t, filepath.Join(filesYaziDraculaRoot(env), "flavor.toml")))
	if err != nil {
		t.Fatal(err)
	}
	if installed["mgr"].(map[string]any)["cwd"].(map[string]any)["fg"] != "#8be9fd" || installed["mode"].(map[string]any)["normal_main"].(map[string]any)["bg"] != "#bd93f9" {
		t.Fatal("installed flavor lacks actual Dracula styles")
	}
	if mode, ok := config["mode"].(map[string]any); ok && mode["normal_main"] != nil {
		t.Fatal("previous direct Catppuccin normal mode still overrides Dracula")
	}
}

func TestYaziDraculaPreservesUnownedStylesAndCustomFirst(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "user styles", true: "registered custom"}[custom], func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("yazi", env)
			testutil.Write(t, path, "[mgr]\nsyntect_theme = '/user/custom.tmTheme'\ncwd = { fg = '#89b4fa' }\n")
			port := *catalog.Port("dracula", "yazi").Official
			if custom {
				port.Kind = "custom"
				port.File = filepath.Join(env.Home, "custom.toml")
				testutil.Write(t, port.File, "[status]\nperm_read = { fg = 'custom' }\n")
			} else {
				for _, asset := range shared.YaziDraculaAssets {
					testutil.Write(t, filepath.Join(filesYaziDraculaRoot(env), asset), string(filesTestYaziDraculaFixture(t, asset)))
				}
			}
			testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
				t.Fatal("custom or complete installed flavor triggered acquisition")
				return nil, errors.New("offline")
			})
			if err := filesApplyYazi(port, env); err != nil {
				t.Fatal(err)
			}
			config, err := shared.TOML(testutil.Read(t, path))
			if err != nil {
				t.Fatal(err)
			}
			mgr := config["mgr"].(map[string]any)
			if mgr["syntect_theme"] != "/user/custom.tmTheme" || mgr["cwd"].(map[string]any)["fg"] != "#89b4fa" {
				t.Fatal("unowned syntax or style override was deleted")
			}
			if custom && config["flavor"] != nil {
				t.Fatal("registered custom port was overridden by official selection")
			}
		})
	}
}
func TestTokyoNightYaziInstallsAuthorFlavorWithoutOverwritingLayout(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 2, 3]\ncwd = { italic = true }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	testutil.Write(t, filepath.Join(filepath.Dir(path), "yazi.toml"), "# untouched\n")
	const flavor = "[mgr]\ncwd = { fg = '#a9b1d6' }\n[filetype]\nrules = [{ name = '*', fg = '#c0caf5' }]\n"
	const syntax = "<?xml version=\"1.0\"?><plist><dict/></plist>"
	if err := filesInstallTokyoNightYazi(env, catalog.Port("tokyo-night", "yazi").Official.Source, []byte(flavor), []byte(syntax)); err != nil {
		t.Fatal(err)
	}
	root := filesYaziFlavorRoot("tokyo-night", env)
	if string(testutil.Read(t, filepath.Join(root, "flavor.toml"))) != flavor || string(testutil.Read(t, filepath.Join(root, "tmtheme.xml"))) != syntax {
		t.Fatal("installed flavor differs from verified author files")
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	selection := config["flavor"].(map[string]any)
	if selection["dark"] != "tokyo-night" || selection["light"] != "tokyo-night" || selection["use"] != nil || config["mgr"].(map[string]any)["ratio"] == nil || config["icon"] == nil {
		t.Fatalf("selection discarded unrelated settings: %#v", config)
	}
	if got := string(testutil.Read(t, filepath.Join(filepath.Dir(path), "yazi.toml"))); got != "# untouched\n" {
		t.Fatalf("rewrote yazi.toml: %q", got)
	}
	first := testutil.Read(t, path)
	if err := filesInstallTokyoNightYazi(env, catalog.Port("tokyo-night", "yazi").Official.Source, []byte(flavor), []byte(syntax)); err != nil || !bytes.Equal(first, testutil.Read(t, path)) {
		t.Fatalf("flavor reapplication changed selection: %v", err)
	}
}

func TestTokyoNightYaziRejectsInvalidSyntaxBeforeWriting(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\ndark = 'old'\n")
	for _, bad := range []string{"", "<not-a-plist/>", "<plist>"} {
		if err := filesInstallTokyoNightYazi(env, catalog.Port("tokyo-night", "yazi").Official.Source, []byte("[mgr]\ncwd = { fg = '#a9b1d6' }\n"), []byte(bad)); err == nil {
			t.Fatalf("accepted invalid syntax %q", bad)
		}
		if string(testutil.Read(t, path)) != "[flavor]\ndark = 'old'\n" {
			t.Fatal("bad syntax changed active theme")
		}
	}
}
func TestTokyoNightOfficialYaziCannotReplaceExistingFlavor(t *testing.T) {
	env := testutil.Env(t)
	root := filesYaziFlavorRoot("tokyo-night", env)
	testutil.Write(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#abcdef' }\n")
	if err := filesInstallTokyoNightYazi(env, catalog.Port("tokyo-night", "yazi").Official.Source, []byte("[mgr]\ncwd = { fg = '#a9b1d6' }\n"), []byte("<plist><dict/></plist>")); err == nil {
		t.Fatal("replaced an independently installed partial flavor")
	}
	if got := string(testutil.Read(t, filepath.Join(root, "flavor.toml"))); !strings.Contains(got, "#abcdef") {
		t.Fatalf("existing flavor changed: %q", got)
	}
}
func TestRosePineYaziNativeFlavorAndFailedOfficialInstall(t *testing.T) {
	env := testutil.Env(t)
	port, ok, err := Adapter().Resolve(catalog.Port("rose-pine", "yazi"), env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("missing official flavor: %#v, %v", port, err)
	}
	path := shared.Path("yazi", env)
	original := "[flavor]\ndark = 'old'\n[mgr]\nratio = [1, 2, 3]\n"
	testutil.Write(t, path, original)
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })
	if err := filesApplyRosePineYazi(port, env); err == nil || string(testutil.Read(t, path)) != original {
		t.Fatalf("failed Yazi install modified theme: %v", err)
	}
	root := filesYaziFlavorRoot("rose-pine", env)
	testutil.Write(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#9ccfd8' }\n")
	testutil.Write(t, filepath.Join(root, "tmtheme.xml"), "<plist><dict/></plist>")
	port, ok, err = Adapter().Resolve(catalog.Port("rose-pine", "yazi"), env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") {
		t.Fatalf("installed flavor not native: %#v, %v", port, err)
	}
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	flavor := config["flavor"].(map[string]any)
	if flavor["dark"] != "rose-pine" || flavor["light"] != "rose-pine" || config["mgr"].(map[string]any)["ratio"] == nil {
		t.Fatalf("lost Yazi settings: %#v", config)
	}
	if _, ok, err := Adapter().Resolve(catalog.Port("rose-pine-dawn", "yazi"), env); ok || err != nil {
		t.Fatalf("unregistered Yazi variant accepted: %v", err)
	}
}

func TestYaziRemoteCustomImportsOnlyStylesAndManagedSyntax(t *testing.T) {
	env := testutil.Env(t)
	if !Adapter().Remote {
		t.Fatal("yazi adapter does not accept remote themes")
	}
	path := shared.Path("yazi", env)
	testutil.Write(t, path, "[flavor]\ndark = 'existing'\nlight = 'existing'\n[mgr]\nratio = [1, 3, 4]\ncwd = { italic = true }\n[status]\nperm_read = { bg = '#123456' }\n[plugin]\nkeep = 'yes'\n")
	neighbor := filepath.Join(filepath.Dir(path), "yazi.toml")
	testutil.Write(t, neighbor, "[mgr]\nshow_hidden = true\n")
	sentinel := filepath.Join(env.Home, "executed")
	const themeURL = "https://example.test/themes/aurora.toml"
	const syntaxURL = "https://example.test/themes/aurora.tmTheme"
	theme := "[mgr]\ncwd = { fg = '#abcdef' }\nsyntect_theme = '/tmp/untrusted.tmTheme'\nratio = [9, 9, 9]\n[status]\nperm_read = { fg = '#fedcba' }\n[flavor]\ndark = 'untrusted'\n[plugin]\nrun = 'touch " + sentinel + "'\n"
	syntax := "<plist><dict/></plist>"
	requests := map[string]int{}
	testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
		requests[req.URL.String()]++
		var data string
		switch req.URL.String() {
		case themeURL:
			data = theme
		case syntaxURL:
			data = syntax
		default:
			t.Fatalf("unexpected source: %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(data)), Header: make(http.Header)}, nil
	})
	port := model.Port{Kind: "custom", Name: "aurora", URL: themeURL, SyntaxURL: syntaxURL, Source: "author's site"}
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	config, err := shared.TOML(testutil.Read(t, path))
	if err != nil {
		t.Fatal(err)
	}
	mgr := config["mgr"].(map[string]any)
	cwd := mgr["cwd"].(map[string]any)
	status := config["status"].(map[string]any)["perm_read"].(map[string]any)
	managed := filepath.Join(filepath.Dir(path), "themeset-custom-aurora.tmTheme")
	if mgr["syntect_theme"] != managed || cwd["fg"] != "#abcdef" || cwd["italic"] != true || fmt.Sprint(mgr["ratio"]) != "[1 3 4]" || status["fg"] != "#fedcba" || status["bg"] != "#123456" || config["flavor"].(map[string]any)["dark"] != "" || config["flavor"].(map[string]any)["light"] != "" || config["plugin"].(map[string]any)["keep"] != "yes" || requests[themeURL] != 1 || requests[syntaxURL] != 1 {
		t.Fatalf("remote Yazi theme modified unrelated settings: %#v, requests=%v", config, requests)
	}
	if string(testutil.Read(t, managed)) != syntax || string(testutil.Read(t, neighbor)) != "[mgr]\nshow_hidden = true\n" {
		t.Fatal("managed syntax or user settings changed unexpectedly")
	}
	if _, err := os.Stat(sentinel); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("remote plugin code executed")
	}
	before := testutil.Read(t, path)
	syntax = "<html>not a plist</html>"
	if err := Adapter().Apply(port, env); err == nil || !bytes.Equal(testutil.Read(t, path), before) || string(testutil.Read(t, managed)) != "<plist><dict/></plist>" {
		t.Fatal("invalid updated syntax replaced active Yazi theme or managed syntax")
	}
}

func TestYaziRemoteCustomRejectsUnsafeAssetsWithoutChangingSettings(t *testing.T) {
	for _, tc := range []struct {
		name, theme, syntax, syntaxURL, portName string
	}{
		{name: "missing syntax", theme: "[mgr]\ncwd = { fg = '#123456' }\n", portName: "aurora"},
		{name: "invalid XML", theme: "[mgr]\ncwd = { fg = '#123456' }\n", syntax: "<html/>", syntaxURL: "https://example.test/syntax", portName: "aurora"},
		{name: "invalid TOML", theme: "[mgr\ncwd =", syntax: "<plist><dict/></plist>", syntaxURL: "https://example.test/syntax", portName: "aurora"},
		{name: "invalid style", theme: "[mgr]\ncwd = { fg = '$(touch /tmp/unsafe)' }\n", syntax: "<plist><dict/></plist>", syntaxURL: "https://example.test/syntax", portName: "aurora"},
		{name: "settings only", theme: "[plugin]\nrun = 'bad'\n[mgr]\nratio = [1, 2, 3]\n", syntax: "<plist><dict/></plist>", syntaxURL: "https://example.test/syntax", portName: "aurora"},
		{name: "unsafe name", theme: "[mgr]\ncwd = { fg = '#123456' }\n", syntax: "<plist><dict/></plist>", syntaxURL: "https://example.test/syntax", portName: "../unsafe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("yazi", env)
			original := "[flavor]\ndark = 'existing'\n[mgr]\ncwd = { fg = '#aabbcc' }\n"
			testutil.Write(t, path, original)
			testutil.AssetClient(t, func(req *http.Request) (*http.Response, error) {
				var data string
				switch req.URL.String() {
				case "https://example.test/theme":
					data = tc.theme
				case "https://example.test/syntax":
					data = tc.syntax
				default:
					t.Fatalf("unexpected remote source %s", req.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(data)), Header: make(http.Header)}, nil
			})
			port := model.Port{Kind: "custom", Name: tc.portName, URL: "https://example.test/theme", SyntaxURL: tc.syntaxURL}
			if err := Adapter().Apply(port, env); err == nil || string(testutil.Read(t, path)) != original {
				t.Fatalf("invalid remote theme changed active Yazi settings: %v", err)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), "themeset-custom-aurora.tmTheme")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid theme installed syntax")
			}
		})
	}
}
