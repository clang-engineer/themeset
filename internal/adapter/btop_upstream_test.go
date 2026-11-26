package adapter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestBtopOfficialThemesFetchCurrentBranch(t *testing.T) {
	for _, name := range []string{"dracula", "gruvbox", "nord", "catppuccin-mocha"} {
		t.Run(name, func(t *testing.T) {
			env := filesTestEnv(t)
			configPath := filesPath("btop", env)
			filesTestWrite(t, configPath, "# keep this setting\nupdate_ms = 2000\ncolor_theme = \"old\"\n")
			port, ok, err := filesResolveBtopUpstream(name)
			if name == "catppuccin-mocha" {
				port, ok, err = filesResolveBtopCatppuccin()
			}
			if err != nil || !ok || port.Kind != "official" {
				t.Fatalf("resolve %q: %#v, ok=%t err=%v", name, port, ok, err)
			}
			license := filesBtopUpstreamLicense
			if name == "catppuccin-mocha" {
				license = filesBtopCatppuccinLicense
			}
			color := "#112233"
			requests := 0
			ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
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
			if !strings.Contains(string(filesTestRead(t, configPath)), "update_ms = 2000") ||
				!strings.Contains(string(filesTestRead(t, themePath)), color) || requests != 2 {
				t.Fatalf("official btop theme not applied with settings preserved: requests=%d", requests)
			}
			color = "#445566"
			if err := filesApplyBtop(port, env); err != nil {
				t.Fatal(err)
			}
			if requests != 4 || !strings.Contains(string(filesTestRead(t, themePath)), color) {
				t.Fatal("reapply did not fetch current upstream theme and license")
			}
		})
	}
}

func TestBtopOfficialDownloadFailurePreservesSettings(t *testing.T) {
	for _, name := range []string{"dracula", "catppuccin-mocha"} {
		for _, failure := range []string{"offline", "invalid theme", "license"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				env := filesTestEnv(t)
				path := filesPath("btop", env)
				const original = "color_theme = \"precious\"\nupdate_ms = 2000\n"
				filesTestWrite(t, path, original)
				port, ok, err := filesResolveBtopUpstream(name)
				if name == "catppuccin-mocha" {
					port, ok, err = filesResolveBtopCatppuccin()
				}
				if err != nil || !ok || port.Kind != "official" {
					t.Fatalf("resolve: %#v, ok=%t err=%v", port, ok, err)
				}
				license := filesBtopUpstreamLicense
				if name == "catppuccin-mocha" {
					license = filesBtopCatppuccinLicense
				}
				ezaDraculaClient(t, func(r *http.Request) (*http.Response, error) {
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
				if !bytes.Equal(filesTestRead(t, path), []byte(original)) {
					t.Fatal("failed download changed active btop settings")
				}
			})
		}
	}
}

func TestBtopOfficialSourceCannotBeSubstituted(t *testing.T) {
	env := filesTestEnv(t)
	if _, err := filesAcquireBtopUpstream(model.Port{Kind: "official", Name: "dracula", Source: "https://example.org/theme.theme"}, env); err == nil {
		t.Fatal("accepted unregistered btop source")
	}
}
