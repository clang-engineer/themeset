package yazi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOfficialYaziFlavorRefreshPreservesExtraFilesAndRejectsIndependentChanges(t *testing.T) {
	root := filepath.Join(t.TempDir(), "flavors", "sample.yazi")
	source := "https://example.org/theme/main"
	first := map[string][]byte{"flavor.toml": []byte("[mgr]\ncwd = { fg = '#111111' }\n"), "tmtheme.xml": []byte("<plist/>")}
	if err := filesInstallOfficialYaziFlavor(root, source, first); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "user.txt"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	updated := map[string][]byte{"flavor.toml": []byte("[mgr]\ncwd = { fg = '#222222' }\n"), "tmtheme.xml": []byte("<plist/>")}
	if err := filesInstallOfficialYaziFlavor(root, source, updated); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "flavor.toml")); err != nil || string(got) != string(updated["flavor.toml"]) {
		t.Fatalf("owned flavor did not refresh: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(root, "user.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("unrelated file lost: %q %v", got, err)
	}
	if err := filesInstallOfficialYaziFlavor(root, "https://example.org/other/main", first); err == nil {
		t.Fatal("replaced a flavor with a different source marker")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "flavor.toml")); string(got) != string(updated["flavor.toml"]) {
		t.Fatal("rejected source changed the active flavor")
	}
}
