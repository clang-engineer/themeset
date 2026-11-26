package btop

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

func TestBtopRemoteCustomTheme(t *testing.T) {
	if !Adapter().Remote {
		t.Fatal("btop must advertise remote custom themes")
	}
	env := testutil.Env(t)
	path := shared.Path("btop", env)
	const original = "color_theme = \"precious\"\nupdate_ms = 2000\n"
	testutil.Write(t, path, original)
	const url = "https://example.com/custom.theme"
	port := model.Port{Kind: "custom", URL: url, Source: "original theme author"}
	body := "theme[main_fg]=\"#112233\"\n"
	testutil.AssetClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != url {
			t.Fatalf("unexpected theme URL: %s", r.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	body = "theme[main_fg]=\"#112233\"\n$(touch /tmp/untrusted)\n"
	if err := Adapter().Apply(port, env); err == nil {
		t.Fatal("accepted executable text in remote theme")
	}
	if string(testutil.Read(t, path)) != original {
		t.Fatal("invalid theme modified btop settings")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "themes", "themeset.theme")); !os.IsNotExist(err) {
		t.Fatalf("invalid theme wrote native asset: %v", err)
	}
	body = "theme[main_fg]=\"#112233\"\n"
	if err := Adapter().Apply(port, env); err != nil {
		t.Fatal(err)
	}
	if got := string(testutil.Read(t, path)); !strings.Contains(got, "color_theme = \"themeset\"") || !strings.Contains(got, "update_ms = 2000") {
		t.Fatalf("unexpected btop settings: %s", got)
	}
	if string(testutil.Read(t, filepath.Join(filepath.Dir(path), "themes", "themeset.theme"))) != body {
		t.Fatal("remote asset was not installed as a native theme")
	}
}
