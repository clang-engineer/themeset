package shared_test

import (
	"path/filepath"
	"testing"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func TestFileThemePathsRespectExplicitAndNativeOverrides(t *testing.T) {
	for _, row := range []struct{ id, key, file string }{
		{"eza", "EZA_CONFIG_DIR", "theme.yml"},
		{"yazi", "YAZI_CONFIG_HOME", "theme.toml"},
	} {
		t.Run(row.id, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv(row.key, directory)
			env := model.Environment{Home: t.TempDir(), OS: "darwin"}
			if actual := shared.Path(row.id, env); actual != filepath.Join(directory, row.file) {
				t.Fatalf("native override ignored: %s", actual)
			}
			destination := filepath.Join(t.TempDir(), row.file)
			env.Paths = map[string]string{row.id: destination}
			if actual := shared.Path(row.id, env); actual != destination {
				t.Fatalf("explicit destination ignored: %s", actual)
			}
		})
	}
}
