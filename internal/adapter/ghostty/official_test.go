package ghostty

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
)

func sampleOfficialTheme(color string) string {
	var theme strings.Builder
	for i := range 16 {
		fmt.Fprintf(&theme, "palette = %d=%s\n", i, color)
	}
	fmt.Fprintf(&theme, "background = %s\nforeground = %s\n", color, color)
	return theme.String()
}

func TestGhosttyOfficialAcceptsPlainHexColors(t *testing.T) {
	theme := strings.ReplaceAll(sampleOfficialTheme("#112233"), "#112233", "112233")
	theme += "cursor-color = abcdef\nsplit-divider-color = #ABCDEF\n"
	if err := validateOfficialTheme([]byte(theme)); err != nil {
		t.Fatalf("upstream-style plain hex colors rejected: %v", err)
	}
}

func TestGhosttyOfficialAcceptsExtendedPalette(t *testing.T) {
	theme := sampleOfficialTheme("#112233") + "palette = 16=#f4a261\npalette = 255=#abcdef\n"
	if err := validateOfficialTheme([]byte(theme)); err != nil {
		t.Fatalf("valid extended palette rejected: %v", err)
	}
	for _, extra := range []string{"palette = 256=#abcdef\n", "palette = 16=#abcdef\n", "palette = 17=invalid\n"} {
		if err := validateOfficialTheme([]byte(theme + extra)); err == nil {
			t.Fatalf("accepted invalid extended palette entry %q", extra)
		}
	}
}

func TestGhosttyOfficialThemesInstallAndRefresh(t *testing.T) {
	for _, theme := range []string{"catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha", "dracula", "rose-pine", "tokyo-night"} {
		t.Run(theme, func(t *testing.T) {
			env := testutil.Env(t)
			config := nativeGhosttyPath(env)
			testutil.Write(t, config, "# keep this\nfont-size = 14\ntheme = \"old\"\n")
			spec := catalog.Port(theme, "ghostty")
			if spec.Official == nil {
				t.Fatalf("missing official %s declaration", theme)
			}
			spec.Native = nil
			port, ok, err := Adapter().Resolve(spec, env)
			if err != nil || !ok || port != *spec.Official {
				t.Fatalf("official fallback: %#v, ok=%t, err=%v", port, ok, err)
			}
			color, requests := "#112233", 0
			testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
				requests++
				if request.URL.String() != port.Source {
					t.Fatalf("unexpected source: %s", request.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sampleOfficialTheme(color)))}, nil
			})
			if err := Adapter().Apply(port, env); err != nil {
				t.Fatal(err)
			}
			asset := filepath.Join(filepath.Dir(config), "themes", "themeset-"+theme)
			if data := string(testutil.Read(t, config)); !strings.Contains(data, "font-size = 14") || !strings.Contains(data, "theme = "+strconv.Quote(asset)) {
				t.Fatalf("theme not selected without changing other settings: %q", data)
			}
			if data := string(testutil.Read(t, asset)); data != sampleOfficialTheme(color) || requests != 1 {
				t.Fatalf("asset not installed: %q (%d requests)", data, requests)
			}
			color = "#445566"
			if err := Adapter().Apply(port, env); err != nil {
				t.Fatal(err)
			}
			if data := string(testutil.Read(t, asset)); data != sampleOfficialTheme(color) || requests != 2 {
				t.Fatalf("asset not refreshed from upstream: %q (%d requests)", data, requests)
			}
		})
	}
}

func TestGhosttyOfficialRejectsForgedMetadataBeforeDownload(t *testing.T) {
	env := testutil.Env(t)
	config := nativeGhosttyPath(env)
	const original = "font-size = 14\ntheme = \"old\"\n"
	testutil.Write(t, config, original)
	port := *catalog.Port("dracula", "ghostty").Official
	testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
		t.Fatalf("fetched unregistered Ghostty source: %s", request.URL)
		return nil, fmt.Errorf("unexpected download")
	})
	for _, mutate := range []func(){
		func() { port.Source = "https://example.invalid/theme" },
		func() { port.Name = "../outside" },
		func() { port.Format = "unsafe" },
	} {
		port = *catalog.Port("dracula", "ghostty").Official
		mutate()
		if err := Adapter().Apply(port, env); err == nil {
			t.Fatal("accepted unregistered official metadata")
		}
		if data := string(testutil.Read(t, config)); data != original {
			t.Fatalf("forged metadata changed config: %q", data)
		}
	}
}

func TestGhosttyOfficialInvalidThemeLeavesPreviousSelection(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"unsupported setting", sampleOfficialTheme("#112233") + "exec = dangerous\n"},
		{"empty", ""},
		{"config inclusion", sampleOfficialTheme("#112233") + "config-file = /tmp/evil\n"},
		{"duplicate color", sampleOfficialTheme("#112233") + "foreground = #ffffff\n"},
		{"incomplete palette", strings.Replace(sampleOfficialTheme("#112233"), "palette = 15=#112233\n", "", 1)},
		{"invalid hex", strings.Replace(sampleOfficialTheme("#112233"), "foreground = #112233", "foreground = #112233;exec", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.Env(t)
			config := nativeGhosttyPath(env)
			const original = "theme = \"old\"\n"
			testutil.Write(t, config, original)
			port := *catalog.Port("dracula", "ghostty").Official
			asset := filepath.Join(filepath.Dir(config), "themes", "themeset-"+port.Name)
			testutil.Write(t, asset, sampleOfficialTheme("#445566"))
			testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})
			if err := Adapter().Apply(port, env); err == nil {
				t.Fatal("invalid Ghostty theme accepted")
			}
			if got := string(testutil.Read(t, config)); got != original {
				t.Fatalf("invalid theme changed selection: %q", got)
			}
			if got := string(testutil.Read(t, asset)); got != sampleOfficialTheme("#445566") {
				t.Fatal("invalid theme overwrote previously installed asset")
			}
		})
	}
}

func TestGhosttyOfficialOfflineLeavesPreviousSelection(t *testing.T) {
	env := testutil.Env(t)
	config := nativeGhosttyPath(env)
	const original = "theme = \"old\"\n"
	testutil.Write(t, config, original)
	port := *catalog.Port("dracula", "ghostty").Official
	testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("offline")
	})
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("offline official source was accepted")
	}
	if got := string(testutil.Read(t, config)); got != original {
		t.Fatalf("offline apply changed selection: %q", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(config), "themes", "themeset-"+port.Name)); !os.IsNotExist(err) {
		t.Fatalf("offline apply installed an asset: %v", err)
	}
}

func TestGhosttyOfficialUsesCustomConfigDirectory(t *testing.T) {
	env := testutil.Env(t)
	env.Paths["ghostty"] = filepath.Join(env.Home, "custom", "config")
	port := *catalog.Port("dracula", "ghostty").Official
	testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sampleOfficialTheme("#123456")))}, nil
	})
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(env.Home, "custom", "themes", "themeset-"+port.Name)
	if got := string(testutil.Read(t, env.Paths["ghostty"])); !strings.Contains(got, "theme = "+strconv.Quote(asset)) {
		t.Fatalf("custom Ghostty config does not select managed theme: %q", got)
	}
	if got := string(testutil.Read(t, asset)); got != sampleOfficialTheme("#123456") {
		t.Fatalf("custom Ghostty managed theme missing: %q", got)
	}
}

func TestGhosttyOfficialRejectsManagedThemeSymlink(t *testing.T) {
	env := testutil.Env(t)
	env.Paths["ghostty"] = filepath.Join(env.Home, "custom", "config")
	port := *catalog.Port("dracula", "ghostty").Official
	outside := filepath.Join(t.TempDir(), "outside")
	testutil.Write(t, outside, "keep")
	asset := filepath.Join(filepath.Dir(nativeGhosttyPath(env)), "themes", "themeset-"+port.Name)
	if err := os.MkdirAll(filepath.Dir(asset), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, asset); err != nil {
		t.Fatal(err)
	}
	testutil.AssetClient(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(sampleOfficialTheme("#123456")))}, nil
	})
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("managed theme asset symlink outside home accepted")
	}
	if got := string(testutil.Read(t, outside)); got != "keep" {
		t.Fatalf("external asset changed: %q", got)
	}
}
