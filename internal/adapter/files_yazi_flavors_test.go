package adapter

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

type filesYaziDraculaTransport func(*http.Request) (*http.Response, error)

func (f filesYaziDraculaTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func filesTestYaziDraculaHTTP(t *testing.T, serve func(*http.Request) (*http.Response, error)) {
	t.Helper()
	previous := filesAssetHTTPClient
	filesAssetHTTPClient = &http.Client{Transport: filesYaziDraculaTransport(serve)}
	t.Cleanup(func() { filesAssetHTTPClient = previous })
}

func filesTestYaziDraculaFixture(t *testing.T, name string) []byte {
	t.Helper()
	return filesTestRead(t, filepath.Join("testdata", "yazi-dracula", name))
}

func filesTestYaziDraculaResponse(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	if !strings.HasPrefix(req.URL.String(), filesYaziDraculaRaw) {
		t.Fatalf("unexpected asset source: %s", req.URL)
	}
	name := strings.TrimPrefix(req.URL.String(), filesYaziDraculaRaw)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(filesTestYaziDraculaFixture(t, name))), Header: make(http.Header)}
}

func TestYaziDraculaResolveDoesNotDownload(t *testing.T) {
	env := filesTestEnv(t)
	calls := 0
	filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("network must not be used by resolve")
	})
	port, ok, err := filesResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "official" || port.File != "" || port.Source != filesYaziDraculaSource {
		t.Fatalf("missing registered official flavor: %#v, %v", port, err)
	}
	if calls != 0 {
		t.Fatal("resolver performed acquisition")
	}
	for _, path := range []string{filesYaziDraculaRoot(env), filepath.Join(env.Home, ".cache"), filesPath("yazi", env)} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("resolver wrote %s: %v", path, err)
		}
	}
}

func TestYaziDraculaInvalidSelectionConfigDoesNotAcquire(t *testing.T) {
	for _, original := range []string{"broken = [\n", "flavor = 'not a table'\n"} {
		env := filesTestEnv(t)
		filesTestWrite(t, filesPath("yazi", env), original)
		filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
			t.Fatal("invalid destination config triggered acquisition")
			return nil, errors.New("offline")
		})
		if err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
			t.Fatal("invalid selection configuration accepted")
		}
		if string(filesTestRead(t, filesPath("yazi", env))) != original {
			t.Fatal("invalid selection configuration changed")
		}
		if _, err := os.Stat(filesYaziDraculaRoot(env)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("invalid selection configuration installed a flavor")
		}
	}
}

func TestYaziDraculaAcquiresCompleteFlavorAndActivates(t *testing.T) {
	env := filesTestEnv(t)
	env.Paths["yazi"] = filepath.Join(env.Home, "explicit", "theme.toml")
	path := filesPath("yazi", env)
	filesTestWrite(t, path, "[flavor]\nuse = 'old'\n[mgr]\nratio = [1, 3, 4]\nborder_style = { fg = 'custom' }\n[icon]\nprepend_dirs = [{ name = 'work', text = 'W' }]\n")
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		filesTestWrite(t, filepath.Join(filepath.Dir(path), name), "# untouched "+name+"\n")
	}
	calls := make(map[string]int)
	filesTestYaziDraculaHTTP(t, func(req *http.Request) (*http.Response, error) {
		calls[filepath.Base(req.URL.Path)]++
		return filesTestYaziDraculaResponse(t, req), nil
	})
	port, _, err := filesResolveYazi("dracula", env)
	if err != nil {
		t.Fatal(err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	root := filesYaziDraculaRoot(env)
	for _, asset := range filesYaziDraculaAssets {
		data := filesTestRead(t, filepath.Join(root, asset.name))
		if !bytes.Equal(data, filesTestYaziDraculaFixture(t, asset.name)) || calls[asset.name] != 1 {
			t.Fatalf("asset %s not acquired exactly once from the current branch", asset.name)
		}
	}
	if string(filesTestRead(t, filepath.Join(root, filesYaziSourceMarker))) != filesYaziDraculaSource+"\n" {
		t.Fatal("official flavor provenance was not recorded")
	}
	config, err := filesTOML(filesTestRead(t, path))
	if err != nil {
		t.Fatal(err)
	}
	flavor := config["flavor"].(map[string]any)
	mgr := config["mgr"].(map[string]any)
	if flavor["dark"] != "dracula" || flavor["light"] != "dracula" || flavor["use"] != nil || mgr["ratio"] == nil || mgr["border_style"].(map[string]any)["fg"] != "custom" || config["icon"] == nil {
		t.Fatalf("selection or user configuration changed: %#v", config)
	}
	for _, name := range []string{"yazi.toml", "keymap.toml"} {
		if string(filesTestRead(t, filepath.Join(filepath.Dir(path), name))) != "# untouched "+name+"\n" {
			t.Fatalf("changed %s", name)
		}
	}
	first := filesTestRead(t, path)
	port, ok, err := filesResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "official" {
		t.Fatalf("installed official flavor not recognized: %#v, %v", port, err)
	}
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatal("reapply changed configuration")
	}
	for name, count := range calls {
		if count != 1 {
			t.Fatalf("reapply downloaded %s again", name)
		}
	}
}

func TestYaziDraculaReusesCompatibleInstalledFlavorOffline(t *testing.T) {
	env := filesTestEnv(t)
	root := filesYaziDraculaRoot(env)
	filesTestWrite(t, filepath.Join(root, "flavor.toml"), "[mgr]\ncwd = { fg = '#123456' }\n")
	filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), string(filesTestYaziDraculaFixture(t, "tmtheme.xml")))
	filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("complete native flavor triggered acquisition")
		return nil, errors.New("offline")
	})
	port, ok, err := filesResolveYazi("dracula", env)
	if err != nil || !ok || port.Kind != "native" || port.File != filepath.Join(root, "flavor.toml") || port.Source != filesYaziDraculaSource {
		t.Fatalf("compatible native flavor not recognized: %#v, %v", port, err)
	}
	before := filesTestRead(t, port.File)
	if err := filesApplyYazi(port, env); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, filesTestRead(t, port.File)) {
		t.Fatal("native flavor overwritten")
	}
}

func TestYaziDraculaTreatsUnmarkedOfficialBytesAsNative(t *testing.T) {
	env := filesTestEnv(t)
	root := filesYaziDraculaRoot(env)
	for _, asset := range filesYaziDraculaAssets {
		filesTestWrite(t, filepath.Join(root, asset.name), string(filesTestYaziDraculaFixture(t, asset.name)))
	}
	filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("complete independently installed flavor triggered acquisition")
		return nil, errors.New("offline")
	})
	port, ok, err := filesResolveYazi("dracula", env)
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
			env := filesTestEnv(t)
			path := filesPath("yazi", env)
			original := "[flavor]\ndark = 'keep'\nlight = 'keep'\n[mgr]\ncwd = { fg = 'custom' }\n"
			filesTestWrite(t, path, original)
			root := filesYaziDraculaRoot(env)
			filesTestWrite(t, filepath.Join(root, "flavor.toml"), string(filesTestYaziDraculaFixture(t, "flavor.toml")))
			filesTestWrite(t, filepath.Join(root, "keep.txt"), "user file")
			filesTestYaziDraculaHTTP(t, func(req *http.Request) (*http.Response, error) {
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
			port, _, err := filesResolveYazi("dracula", env)
			if err != nil {
				t.Fatal(err)
			}
			if err := filesApplyYazi(port, env); err == nil {
				t.Fatal("failed acquisition accepted")
			}
			if string(filesTestRead(t, path)) != original || string(filesTestRead(t, filepath.Join(root, "keep.txt"))) != "user file" {
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
			env := filesTestEnv(t)
			root := filesYaziDraculaRoot(env)
			partial := string(filesTestYaziDraculaFixture(t, "flavor.toml"))
			if conflict {
				partial = "[mgr]\ncwd = { fg = 'keep' }\n"
			}
			filesTestWrite(t, filepath.Join(root, "flavor.toml"), partial)
			filesTestWrite(t, filepath.Join(root, "keep.txt"), "user file")
			filesTestWrite(t, filesPath("yazi", env), "[flavor]\ndark = 'keep'\n")
			before := filesTestRead(t, filesPath("yazi", env))
			calls := 0
			filesTestYaziDraculaHTTP(t, func(req *http.Request) (*http.Response, error) {
				calls++
				return filesTestYaziDraculaResponse(t, req), nil
			})
			err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env)
			if conflict {
				if err == nil || calls != 1 || !bytes.Equal(before, filesTestRead(t, filesPath("yazi", env))) {
					t.Fatal("conflicting native partial flavor was not preserved")
				}
				if _, err := os.Stat(filepath.Join(root, filesYaziSourceMarker)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("conflicting partial flavor was marked official")
				}
			} else if err != nil || calls != len(filesYaziDraculaAssets) {
				t.Fatalf("matching partial flavor not completed: calls=%d, %v", calls, err)
			}
			if string(filesTestRead(t, filepath.Join(root, "keep.txt"))) != "user file" || string(filesTestRead(t, filepath.Join(root, "flavor.toml"))) != partial {
				t.Fatal("existing partial flavor data changed")
			}
		})
	}
}

func TestYaziDraculaRejectsMalformedDownloadsWithoutChangingSelection(t *testing.T) {
	for _, failure := range []string{"invalid TOML", "missing manager", "invalid syntax", "empty license"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath("yazi", env)
			original := "[flavor]\ndark = 'keep'\n"
			filesTestWrite(t, path, original)
			filesTestYaziDraculaHTTP(t, func(req *http.Request) (*http.Response, error) {
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
			if err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
				t.Fatal("invalid upstream flavor was installed")
			}
			if string(filesTestRead(t, path)) != original {
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
			env := filesTestEnv(t)
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
			filesTestWrite(t, filepath.Join(root, "flavor.toml"), flavor)
			filesTestWrite(t, filepath.Join(root, "tmtheme.xml"), syntax)
			original := "[flavor]\ndark = 'keep'\n"
			filesTestWrite(t, filesPath("yazi", env), original)
			filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
				t.Fatal("malformed native flavor triggered acquisition")
				return nil, errors.New("offline")
			})
			if _, _, err := filesResolveYazi("dracula", env); err == nil {
				t.Fatal("malformed installed source accepted by resolver")
			}
			if err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env); err == nil {
				t.Fatal("malformed installed source accepted by apply")
			}
			if string(filesTestRead(t, filesPath("yazi", env))) != original {
				t.Fatal("malformed source changed selection")
			}
		})
	}
}

func TestYaziDraculaRemovesOnlyOwnedPreviousCatppuccinLeaves(t *testing.T) {
	env := filesTestEnv(t)
	cat, ok := filesOfficialYazi("catppuccin-mocha")
	if !ok {
		t.Fatal("official Catppuccin port missing")
	}
	if err := filesApplyYazi(cat, env); err != nil {
		t.Fatal(err)
	}
	path := filesPath("yazi", env)
	if err := filesEditTOML(path, func(config map[string]any) error {
		mgr := config["mgr"].(map[string]any)
		mgr["cwd"].(map[string]any)["italic"] = true
		mgr["border_style"].(map[string]any)["fg"] = "user-custom"
		mgr["ratio"] = []int{1, 3, 4}
		config["icon"] = map[string]any{"prepend_dirs": []map[string]any{{"name": "work", "text": "W"}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	filesTestYaziDraculaHTTP(t, func(req *http.Request) (*http.Response, error) {
		return filesTestYaziDraculaResponse(t, req), nil
	})
	if err := filesApplyYazi(model.Port{Kind: "official", Name: "dracula"}, env); err != nil {
		t.Fatal(err)
	}
	config, err := filesTOML(filesTestRead(t, path))
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
	installed, err := filesTOML(filesTestRead(t, filepath.Join(filesYaziDraculaRoot(env), "flavor.toml")))
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
			env := filesTestEnv(t)
			path := filesPath("yazi", env)
			filesTestWrite(t, path, "[mgr]\nsyntect_theme = '/user/custom.tmTheme'\ncwd = { fg = '#89b4fa' }\n")
			port := model.Port{Kind: "official", Name: "dracula"}
			if custom {
				port.Kind = "custom"
				port.File = filepath.Join(env.Home, "custom.toml")
				filesTestWrite(t, port.File, "[status]\nperm_read = { fg = 'custom' }\n")
			} else {
				for _, asset := range filesYaziDraculaAssets {
					filesTestWrite(t, filepath.Join(filesYaziDraculaRoot(env), asset.name), string(filesTestYaziDraculaFixture(t, asset.name)))
				}
			}
			filesTestYaziDraculaHTTP(t, func(*http.Request) (*http.Response, error) {
				t.Fatal("custom or complete installed flavor triggered acquisition")
				return nil, errors.New("offline")
			})
			if err := filesApplyYazi(port, env); err != nil {
				t.Fatal(err)
			}
			config, err := filesTOML(filesTestRead(t, path))
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
