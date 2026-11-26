package btop

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/catalog"
)

func TestBtopOfficialThemesFetchCurrentBranch(t *testing.T) {
	for _, name := range []string{"dracula", "gruvbox", "nord", "catppuccin-mocha", "rose-pine", "tokyo-night"} {
		t.Run(name, func(t *testing.T) {
			env := testutil.Env(t)
			configPath := shared.Path("btop", env)
			testutil.Write(t, configPath, "# keep this setting\nupdate_ms = 2000\ncolor_theme = \"old\"\n")
			spec := catalog.Port(name, "btop")
			spec.Native = nil
			port, ok, err := filesResolveBtop(spec, env)
			if err != nil || !ok || port.Kind != "official" {
				t.Fatalf("resolve %q: %#v, ok=%t err=%v", name, port, ok, err)
			}
			license := port.LicenseURL
			expectedRequests := 1
			if license != "" {
				expectedRequests++
			}
			color := "#112233"
			requests := 0
			testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
				requests++
				var body string
				switch r.URL.String() {
				case port.Source:
					body = "theme[main_fg]=\"" + color + "\"\n"
				case license:
					body = "Official theme license\n"
				default:
					t.Fatalf("unexpected btop download: %s", r.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			if requests != 0 {
				t.Fatal("resolution downloaded a theme")
			}
			if err := filesApplyBtop(port, env); err != nil {
				t.Fatal(err)
			}
			themePath := filepath.Join(filepath.Dir(configPath), "themes", port.Name+".theme")
			if !strings.Contains(string(testutil.Read(t, configPath)), "update_ms = 2000") ||
				!strings.Contains(string(testutil.Read(t, themePath)), color) || requests != expectedRequests {
				t.Fatalf("official btop theme not applied with settings preserved: requests=%d", requests)
			}
			color = "#445566"
			if err := filesApplyBtop(port, env); err != nil {
				t.Fatal(err)
			}
			if requests != 2*expectedRequests || !strings.Contains(string(testutil.Read(t, themePath)), color) {
				t.Fatal("reapply did not fetch current upstream theme and optional license")
			}
		})
	}
}

func TestBtopOfficialDownloadFailurePreservesSettings(t *testing.T) {
	for _, name := range []string{"dracula", "catppuccin-mocha"} {
		for _, failure := range []string{"offline", "invalid theme", "license"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				env := testutil.Env(t)
				path := shared.Path("btop", env)
				const original = "color_theme = \"precious\"\nupdate_ms = 2000\n"
				testutil.Write(t, path, original)
				spec := catalog.Port(name, "btop")
				spec.Native = nil
				port, ok, err := filesResolveBtop(spec, env)
				if err != nil || !ok || port.Kind != "official" {
					t.Fatalf("resolve: %#v, ok=%t err=%v", port, ok, err)
				}
				license := port.LicenseURL
				testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
					if failure == "offline" || failure == "license" && r.URL.String() == license {
						return nil, fmt.Errorf("offline")
					}
					body := "theme[main_fg]=\"#112233\"\n"
					if failure == "invalid theme" {
						body = "execute untrusted shell code\n"
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})
				if err := filesApplyBtop(port, env); err == nil {
					t.Fatal("failed download applied")
				}
				if !bytes.Equal(testutil.Read(t, path), []byte(original)) {
					t.Fatal("failed download changed active btop settings")
				}
			})
		}
	}
}

func TestBtopOfficialSourcesCannotBeSubstituted(t *testing.T) {
	env := testutil.Env(t)
	path := shared.Path("btop", env)
	const original = "color_theme = \"precious\"\nupdate_ms = 2000\n"
	testutil.Write(t, path, original)
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unregistered btop source was fetched: %s", r.URL)
	})
	for _, field := range []string{"source", "license"} {
		t.Run(field, func(t *testing.T) {
			spec := catalog.Port("dracula", "btop")
			port := *spec.Official
			if field == "source" {
				port.Source = "https://example.org/theme.theme"
			} else {
				port.LicenseURL = "https://example.org/LICENSE"
			}
			if err := filesApplyBtop(port, env); err == nil {
				t.Fatal("accepted unregistered btop source")
			}
			if requests != 0 {
				t.Fatalf("fetched unregistered btop source %d times", requests)
			}
			if got := string(testutil.Read(t, path)); got != original {
				t.Fatalf("unregistered source changed settings: %q", got)
			}
		})
	}
}
