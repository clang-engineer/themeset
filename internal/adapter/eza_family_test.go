package adapter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ezaGruvboxFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/eza-gruvbox-dark.yml")
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
	env := filesTestEnv(t)
	path := filepath.Join(env.Home, "custom", "eza", "theme.yml")
	env.Paths["eza"] = path
	filesTestWrite(t, path, "filenames:\n  Makefile: {icon: {glyph: KEEP}}\nfilekinds:\n  directory: {is_italic: true}\nextensions:\n  mine: {filename: {foreground: red}}\n")
	requests := 0
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		var body []byte
		switch r.URL.String() {
		case filesEzaGruvboxSource:
			body = fixture
		case filesEzaFamilyLicenseSource:
			body = []byte(ezaDraculaTestLicense)
		default:
			t.Fatalf("unexpected eza asset request %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	port, ok, err := filesResolveEzaFamily("gruvbox", env)
	if err != nil || !ok || port.Kind != "official" || port.Source != filesEzaGruvboxSource || requests != 0 {
		t.Fatalf("resolve: %#v, ok=%t, err=%v, requests=%d", port, ok, err, requests)
	}
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("downloaded %d assets, want theme and license", requests)
	}
	first := filesTestRead(t, path)
	config, err := filesYAML(first)
	if err != nil {
		t.Fatal(err)
	}
	directory := config["filekinds"].(map[string]any)["directory"].(map[string]any)
	if directory["foreground"] != "#83a598" || directory["is_italic"] != true || config["filenames"].(map[string]any)["Makefile"].(map[string]any)["icon"].(map[string]any)["glyph"] != "KEEP" || config["extensions"].(map[string]any)["mine"] == nil {
		t.Fatalf("theme did not preserve user settings: %#v", config)
	}
	// A second application fetches the current upstream branch again.
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		body := fixture
		if r.URL.String() == filesEzaFamilyLicenseSource {
			body = []byte(ezaDraculaTestLicense)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 4 || !bytes.Equal(first, filesTestRead(t, path)) {
		t.Fatal("reapply failed to fetch current theme or changed effective theme")
	}
}

func TestEzaFamilyInitialAcquisitionFailurePreservesTheme(t *testing.T) {
	fixture := ezaGruvboxFixture(t)
	for _, failure := range []string{"offline", "invalid schema", "missing license"} {
		t.Run(failure, func(t *testing.T) {
			env := filesTestEnv(t)
			path := filesPath("eza", env)
			const original = "filenames:\n  precious: {filename: {foreground: red}}\n"
			filesTestWrite(t, path, original)
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
				if failure == "offline" || failure == "missing license" && r.URL.String() == filesEzaFamilyLicenseSource {
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
			if string(filesTestRead(t, path)) != original {
				t.Fatal("failed acquisition mutated active theme")
			}
		})
	}
}

func TestEzaFamilyOfflineReapplyPreservesTheme(t *testing.T) {
	env := filesTestEnv(t)
	path := filesPath("eza", env)
	filesTestWrite(t, path, "filenames:\n  precious: {filename: {foreground: red}}\n")
	port, ok, err := filesResolveEzaFamily("gruvbox", env)
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	fixture := ezaGruvboxFixture(t)
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		body := fixture
		if r.URL.String() == filesEzaFamilyLicenseSource {
			body = []byte(ezaDraculaTestLicense)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})
	if err := filesApplyEza(port, env); err != nil {
		t.Fatal(err)
	}
	applied := filesTestRead(t, path)
	ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline: %s", r.URL)
	})
	if err := filesApplyEza(port, env); err == nil {
		t.Fatal("offline reapply succeeded without fetching current branch")
	}
	if !bytes.Equal(filesTestRead(t, path), applied) {
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
	env := filesTestEnv(t)
	if _, ok, err := filesResolveEzaFamily("nord", env); ok || err != nil {
		t.Fatalf("unverified Nord unexpectedly resolved: ok=%v err=%v", ok, err)
	}
	if _, ok, err := filesResolveEzaFamily("moonfly", env); ok || err != nil {
		t.Fatalf("unverified Moonfly unexpectedly resolved: ok=%v err=%v", ok, err)
	}
	if _, err := filesAcquireEzaFamily("nord", env); err == nil {
		t.Fatal("unregistered eza family theme acquired")
	}
}
