package eza

import (
	"bytes"
	"fmt"
	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestEzaDraculaAcquiresCurrentNativeThemeAndLicense(t *testing.T) {
	env := testutil.Env(t)
	path := filepath.Join(env.Home, "explicit", "theme.yml")
	env.Paths["eza"] = path
	testutil.Write(t, path, "filenames:\n  Makefile:\n    icon:\n      glyph: KEEP\nfilekinds:\n  directory:\n    is_italic: true\nextensions:\n  mine:\n    filename: {foreground: red}\n")
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch r.URL.String() {
		case shared.EzaDraculaSource:
			body = testutil.EzaDraculaTestTheme
		case shared.EzaDraculaLicenseSource:
			body = testutil.EzaDraculaTestLicense
		default:
			t.Fatalf("unexpected asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveEza("dracula", env)
	if err != nil || !ok || port.Kind != "official" || requests != 0 {
		t.Fatalf("resolver: %#v, %v; requests=%d", port, err, requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	first := testutil.Read(t, path)
	config, err := shared.YAML(first)
	if err != nil {
		t.Fatal(err)
	}
	directory := config["filekinds"].(map[string]any)["directory"].(map[string]any)
	if directory["foreground"] != "#8BE9FD" || directory["is_italic"] != true || config["filenames"].(map[string]any)["Makefile"].(map[string]any)["icon"].(map[string]any)["glyph"] != "KEEP" || config["extensions"].(map[string]any)["mine"] == nil {
		t.Fatalf("native rule merge failed: %#v", config)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want theme and license", requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 4 || !bytes.Equal(first, testutil.Read(t, path)) {
		t.Fatal("reapply did not fetch latest theme and license or changed effective theme")
	}
}

func TestEzaDraculaFailureDoesNotMutateActiveTheme(t *testing.T) {
	for _, failure := range []string{"download", "invalid schema", "license"} {
		t.Run(failure, func(t *testing.T) {
			env := testutil.Env(t)
			path := shared.Path("eza", env)
			const original = "filenames:\n  precious: {filename: {foreground: red}}\n"
			testutil.Write(t, path, original)
			testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "download" || (failure == "license" && r.URL.String() == shared.EzaDraculaLicenseSource) {
					return nil, fmt.Errorf("offline")
				}
				body := testutil.EzaDraculaTestTheme
				if failure == "invalid schema" {
					body = "gui: {theme: {}}\n"
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			port, _, _ := filesResolveEza("dracula", env)
			if err := filesApplyEza(port, env); err == nil {
				t.Fatal("expected acquisition failure")
			}
			if string(testutil.Read(t, path)) != original {
				t.Fatal("failure mutated active theme")
			}
		})
	}
}

func TestEzaDraculaRejectsNonNativeThemeData(t *testing.T) {
	for _, bad := range []string{"", "gui:\n  theme: {}\n", "export EZA_COLORS=blue\n", "colourful: true\nfilekinds: broken\n", strings.Replace(testutil.EzaDraculaTestTheme, "directory: {foreground: \"#8BE9FD\"}", "directory: {foreground: false}", 1)} {
		if err := filesValidateEzaDracula([]byte(bad)); err == nil {
			t.Fatalf("accepted non-native YAML: %q", bad)
		}
	}
}
