package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/clang-engineer/themeset/internal/model"
)

func TestCompletionContext(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "custom.json")
	if err := os.WriteFile(config, []byte(`{"themes":{"my-theme":{}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	adapters := []model.Adapter{{ID: "bat"}, {ID: "btop"}, {ID: "fzf"}}
	tests := []struct {
		name  string
		words []string
		want  []string
	}{
		{"command after config", []string{"--config", config, "ap"}, []string{"apply"}},
		{"custom theme after flags", []string{"--config", config, "apply", "--home", home, "my-"}, []string{"my-theme"}},
		{"comma tools exclude selected", []string{"--tools", "bat,b"}, []string{"bat,btop"}},
		{"inline tools", []string{"apply", "dracula", "--tools=bat,f"}, []string{"--tools=bat,fzf"}},
		{"no third positional", []string{"apply", "dracula", "dra"}, nil},
		{"dry run allowed", []string{"apply", "--dry"}, []string{"--dry-run"}},
		{"dry run invalid for check", []string{"check", "--dry"}, nil},
		{"literal stops flag completion", []string{"apply", "--", "--dry"}, nil},
		{"shell selection", []string{"completion", "z"}, []string{"zsh"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := completeWords(test.words, adapters)
			if len(got) == 0 && len(test.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %q; want %q", got, test.want)
			}
		})
	}
}
