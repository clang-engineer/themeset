package app

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
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
	themes := append(append([]string(nil), builtinThemes...), "my-theme")
	sort.Strings(themes)
	tests := []struct {
		name  string
		words []string
		want  []string
	}{
		{"root defaults", []string{""}, []string{"apply", "current", "list"}},
		{"root after global flag", []string{"--json", ""}, []string{"apply", "current", "list"}},
		{"root command prefix includes auxiliary", []string{"c"}, []string{"completion", "current"}},
		{"help prefix", []string{"h"}, []string{"help"}},
		{"version prefix", []string{"v"}, []string{"version"}},
		{"removed check prefix", []string{"ch"}, nil},
		{"removed tools prefix", []string{"t"}, nil},
		{"hidden commands excluded", []string{"__"}, nil},
		{"root options require dash", []string{"-"}, []string{"--config", "--help", "--home", "--json", "--tools", "--version"}},
		{"root has no dry run", []string{"--dry"}, nil},
		{"list subcommand", []string{"list", ""}, []string{"tools"}},
		{"list subcommand prefix", []string{"list", "t"}, []string{"tools"}},
		{"list no themes subcommand", []string{"list", "themes"}, nil},
		{"list tools has no positionals", []string{"list", "tools", ""}, nil},
		{"current has no positionals or bare options", []string{"current", ""}, nil},
		{"apply themes without bare options", []string{"--home", home, "--config", config, "apply", ""}, themes},
		{"apply options after theme", []string{"apply", "dracula", "--dry"}, []string{"--dry-run"}},
		{"apply no bare options after theme", []string{"apply", "dracula", ""}, nil},
		{"command after config", []string{"--config", config, "ap"}, []string{"apply"}},
		{"custom theme after flags", []string{"--config", config, "apply", "--home", home, "my-"}, []string{"my-theme"}},
		{"comma tools exclude selected", []string{"--tools", "bat,b"}, []string{"bat,btop"}},
		{"inline tools", []string{"apply", "dracula", "--tools=bat,f"}, []string{"--tools=bat,fzf"}},
		{"no third positional", []string{"apply", "dracula", "dra"}, nil},
		{"dry run allowed", []string{"apply", "--dry"}, []string{"--dry-run"}},
		{"dry run invalid for list", []string{"list", "--dry"}, nil},
		{"dry run invalid for list tools", []string{"list", "tools", "--dry"}, nil},
		{"dry run invalid for current", []string{"current", "--dry"}, nil},
		{"dry run invalid for completion", []string{"completion", "--dry"}, nil},
		{"literal stops flag completion", []string{"apply", "--", "--dry"}, nil},
		{"literal stops inline tool completion", []string{"apply", "--", "--tools=b"}, nil},
		{"literal stops separate tool completion", []string{"apply", "--", "--tools", "b"}, nil},
		{"shell selection", []string{"completion", "z"}, []string{"zsh"}},
		{"shell choices without options", []string{"completion", ""}, []string{"bash", "fish", "zsh"}},
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
