package atuin

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/adapter/testutil"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestAtuinRemoteCustomTheme(t *testing.T) {
	if !Adapter().Remote {
		t.Fatal("Atuin must advertise remote custom themes")
	}
	env := testutil.Env(t)
	path := shared.Path("atuin", env)
	const original = "enter_accept = false\n[theme]\nname = 'old'\n"
	testutil.Write(t, path, original)
	const url = "https://example.com/atuin.toml"
	port := model.Port{Kind: "custom", Name: "custom-palette", URL: url, Source: "theme author's attribution"}
	body := "[theme]\nname = 'custom-palette'\n[colors]\nAlertInfo = '#112233'\n"
	requests := 0
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.String() != url {
			t.Fatalf("unexpected theme URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	body = "[theme]\nname = 'custom-palette'\n[colors]\nAlertInfo = {not = 'a color'}\n"
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("accepted invalid remote TOML theme")
	}
	if string(testutil.Read(t, path)) != original {
		t.Fatal("invalid theme modified Atuin settings")
	}
	body = "[theme]\nname = 'custom-palette'\n[colors]\nAlertInfo = '#112233'\n"
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	if requests != 2 || !strings.Contains(string(testutil.Read(t, path)), "enter_accept = false") || !strings.Contains(string(testutil.Read(t, path)), "custom-palette") {
		t.Fatal("remote Atuin theme was not applied safely")
	}
	if string(testutil.Read(t, filepath.Join(filesAtuinThemes(env), "custom-palette.toml"))) != body {
		t.Fatal("remote theme was not stored as native TOML")
	}
}

func TestAtuinRemoteRefusesUnownedOrChangedTheme(t *testing.T) {
	env := testutil.Env(t)
	configPath := shared.Path("atuin", env)
	testutil.Write(t, configPath, "[theme]\nname = 'old'\n")
	const source = "https://example.com/atuin.toml"
	port := model.Port{Kind: "custom", Name: "new-theme", URL: source}
	asset := "[theme]\nname = 'new-theme'\n[colors]\nAlertInfo = '#112233'\n"
	testutil.AssetClient(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(asset)), Header: make(http.Header)}, nil
	})
	target := filepath.Join(filesAtuinThemes(env), "new-theme.toml")
	testutil.Write(t, target, "user-owned theme\n")
	if err := Adapter().Apply(port, env); err == nil || string(testutil.Read(t, target)) != "user-owned theme\n" {
		t.Fatalf("overwrote independent Atuin theme: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, target, asset+"# user edit\n")
	if err := Adapter().Apply(port, env); err == nil || string(testutil.Read(t, target)) != asset+"# user edit\n" {
		t.Fatalf("overwrote modified Atuin theme: %v", err)
	}
	config, err := shared.TOML(testutil.Read(t, configPath))
	if err != nil || config["theme"].(map[string]any)["name"] != "new-theme" {
		t.Fatalf("remote failure changed Atuin selection: %v, %#v", err, config)
	}
	asset = "[theme]\nname = 'different'\n[colors]\nAlertInfo = '#112233'\n"
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("accepted remote theme with a different declared name")
	}
}
