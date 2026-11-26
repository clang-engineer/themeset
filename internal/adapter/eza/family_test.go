package eza

import (
	"bytes"
	"fmt"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func filesResolveEzaFamily(name string, env model.Environment) (model.Port, bool, error) {
	if name != "gruvbox" {
		return filesResolveEza(model.ThemePort{}, env)
	}
	return filesResolveEza(catalog.Port(name, "eza"), env)
}

func ezaGruvboxFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../testdata/eza-gruvbox-dark.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := filesValidateEzaFamily(data); err != nil {
		t.Fatalf("fixture is not a native Gruvbox Dark theme: %v", err)
	}
	return data
}

func TestEzaFamilyGruvboxAcquisitionFetchesLatest(t *testing.T) {
	fixture := ezaGruvboxFixture(t)
	env := testutil.Env(t)
	path := filepath.Join(env.Home, "custom", "eza", "theme.yml")
	env.Paths["eza"] = path
	testutil.Write(t, path, "filenames:\n  Makefile: {icon: {glyph: KEEP}}\nfilekinds:\n  directory: {is_italic: true}\nextensions:\n  mine: {filename: {foreground: red}}\n")
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != shared.EzaGruvboxSource {
			t.Fatalf("unexpected eza asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(fixture)), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveEzaFamily("gruvbox", env)
	if err != nil || !ok || port.Kind != "official" || port.Source != shared.EzaGruvboxSource || requests != 0 {
		t.Fatalf("resolve: %#v, ok=%t, err=%v, requests=%d", port, ok, err, requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("downloaded %d assets, want theme", requests)
	}
	first := testutil.Read(t, path)
	config, err := shared.YAML(first)
	if err != nil {
		t.Fatal(err)
	}
	directory := config["filekinds"].(map[string]any)["directory"].(map[string]any)
	if directory["foreground"] != "#83a598" || directory["is_italic"] != true || config["filenames"].(map[string]any)["Makefile"].(map[string]any)["icon"].(map[string]any)["glyph"] != "KEEP" || config["extensions"].(map[string]any)["mine"] == nil {
		t.Fatalf("theme did not preserve user settings: %#v", config)
	}
	// A second application fetches the current upstream branch again.
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != shared.EzaGruvboxSource {
			t.Fatalf("unexpected eza asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(fixture)), Header: make(http.Header)}, nil
	})
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !bytes.Equal(first, testutil.Read(t, path)) {
		t.Fatal("reapply failed to fetch current theme or changed effective theme")
	}
}

func TestEzaFamilyInitialAcquisitionFailurePreservesTheme(t *testing.T) {
	fixture := ezaGruvboxFixture(t)
	for _, failure := range []string{"offline", "invalid schema"} {
		t.Run(failure, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("eza", env)
			const original = "filenames:\n  precious: {filename: {foreground: red}}\n"
			testutil.Write(t, path, original)
			testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "offline" {
					return nil, fmt.Errorf("offline")
				}
				body := fixture
				if failure == "invalid schema" {
					body = []byte("gui: {theme: {}}\n")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})
			port, ok, err := filesResolveEzaFamily("gruvbox", env)
			if err != nil || !ok {
				t.Fatalf("resolve: %v, %v", ok, err)
			}
			if err := filesApplyEza(port, env); err == nil {
				t.Fatal("expected acquisition error")
			}
			if string(testutil.Read(t, path)) != original {
				t.Fatal("failed acquisition mutated active theme")
			}
		})
	}
}

func TestEzaFamilyOfflineReapplyPreservesTheme(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("eza", env)
	testutil.Write(t, path, "filenames:\n  precious: {filename: {foreground: red}}\n")
	port, ok, err := filesResolveEzaFamily("gruvbox", env)
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	fixture := ezaGruvboxFixture(t)
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != shared.EzaGruvboxSource {
			t.Fatalf("unexpected eza asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(fixture)), Header: make(http.Header)}, nil
	})
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	applied := testutil.Read(t, path)
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline: %s", r.URL)
	})
	if err := filesApplyEza(port, env); err == nil {
		t.Fatal("offline reapply succeeded without fetching current branch")
	}
	if !bytes.Equal(testutil.Read(t, path), applied) {
		t.Fatal("offline reapply mutated active theme")
	}
}
func TestEzaFamilyRejectsWrongSchemaAndUnknownTheme(t *testing.T) {
	fixture := ezaGruvboxFixture(t)
	for _, bad := range []string{"", "gui: {theme: {}}\n", "colourful: true\nfilekinds: broken\n", strings.Replace(string(fixture), "directory: {foreground: \"#83a598\"}", "directory: {foreground: false}", 1), strings.Replace(string(fixture), "directory: {foreground: \"#83a598\"}", "directory: {foreground: \"#nothex\"}", 1)} {
		if err := filesValidateEzaFamily([]byte(bad)); err == nil {
			t.Fatalf("accepted non-native eza colors: %q", bad)
		}
	}
	env := testutil.Env(t)
	if _, ok, err := filesResolveEzaFamily("nord", env); ok || err != nil {
		t.Fatalf("unverified Nord unexpectedly resolved: ok=%v err=%v", ok, err)
	}
	if _, ok, err := filesResolveEzaFamily("moonfly", env); ok || err != nil {
		t.Fatalf("unverified Moonfly unexpectedly resolved: ok=%v err=%v", ok, err)
	}
	if _, err := filesPortData(model.Port{Kind: "official", Name: "nord", Format: "eza-family"}, env); err == nil {
		t.Fatal("unregistered eza family theme acquired")
	}
}
