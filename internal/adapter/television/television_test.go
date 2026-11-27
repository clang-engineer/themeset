package television

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestApplyNativeThemePreservesConfig(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".config", "television", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	original := "# keep\n[ui]\ntheme = 'default' # selected theme\n[input_bar]\nborder_type = \"plain\"\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	port := catalog.Port("catppuccin-mocha", "television").Native
	if len(port) != 1 {
		t.Fatalf("television port not registered: %v", port)
	}
	if err := Adapter().Apply(model.Port{Kind: "native", Name: port[0]}, model.Environment{Home: home, ConfigHome: filepath.Join(home, ".config"), OS: "linux"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# keep", "theme = 'catppuccin' # selected theme", "border_type = \"plain\""} {
		if !strings.Contains(string(got), want) {
			t.Errorf("updated television config lost %q:\n%s", want, got)
		}
	}
}
