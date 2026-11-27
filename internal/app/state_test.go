package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadStateRequiresOneCompleteJSONValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{name: "valid result with whitespace", content: "{\"theme\":\"dracula\",\"tools\":[]}  \n"},
		{name: "multiple values", content: `{"theme":"dracula","tools":[]}{}`, wantErr: true},
		{name: "trailing garbage", content: `{"theme":"dracula","tools":[]} invalid`, wantErr: true},
		{name: "unknown field", content: `{"theme":"dracula","tools":[],"unexpected":true}`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := readState(path)
			if (err != nil) != test.wantErr {
				t.Fatalf("readState() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr && result != nil {
				t.Fatalf("readState() returned a partial result after invalid data: %+v", result)
			}
			if !test.wantErr && (result == nil || result.Theme != "dracula") {
				t.Fatalf("readState() result = %+v", result)
			}
		})
	}
}

func TestSaveStateRefusesSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.json")
	path := filepath.Join(directory, "state.json")
	if err := os.WriteFile(target, []byte("keep me\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := saveState(path, Result{Theme: "dracula"})
	if err == nil || !strings.Contains(err.Error(), "refusing to replace symlink") {
		t.Fatalf("saveState() error = %v, want symlink refusal", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep me\n" {
		t.Fatalf("symlink target changed: %q, %v", data, err)
	}
}
