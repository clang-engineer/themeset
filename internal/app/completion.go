package app

import (
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

//go:embed completions/*
var completionScripts embed.FS

func printCompletion(shell string, stdout, stderr io.Writer) int {
	switch shell {
	case "bash", "zsh", "fish":
	default:
		fmt.Fprintln(stderr, "error: completion expects bash, zsh, or fish")
		return 2
	}
	script, err := completionScripts.ReadFile("completions/" + shell)
	if err == nil {
		_, err = stdout.Write(script)
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

// completeWords never inspects providers or writes app configuration. The last
// word is always the word being completed, including an explicit empty word.
func completeWords(words []string, adapters []model.Adapter) []string {
	if len(words) == 0 {
		return nil
	}
	current := words[len(words)-1]
	prior := words[:len(words)-1]
	pending := ""
	if len(prior) > 0 {
		switch prior[len(prior)-1] {
		case "--config", "--home", "--tools", "-config", "-home", "-tools":
			pending = strings.TrimLeft(prior[len(prior)-1], "-")
			prior = prior[:len(prior)-1]
		}
	}
	prefix := ""
	if pending == "" {
		for _, flag := range []string{"--config=", "--home=", "--tools="} {
			if strings.HasPrefix(current, flag) {
				pending = strings.TrimSuffix(strings.TrimPrefix(flag, "--"), "=")
				prefix = flag
				current = strings.TrimPrefix(current, flag)
				break
			}
		}
	}
	opts, err := parse(prior)
	if err != nil {
		return nil
	}
	var candidates []string
	if pending != "" {
		if pending != "tools" {
			return nil
		}
		parts := strings.Split(current, ",")
		fragment := parts[len(parts)-1]
		selected := parts[:len(parts)-1]
		base := strings.TrimSuffix(current, fragment)
		for _, entry := range adapters {
			used := false
			for _, id := range selected {
				if id == entry.ID {
					used = true
					break
				}
			}
			if !used && strings.HasPrefix(entry.ID, fragment) {
				candidates = append(candidates, prefix+base+entry.ID)
			}
		}
	} else {
		command := ""
		if len(opts.positionals) > 0 {
			command = opts.positionals[0]
		}
		literal := false
		for _, word := range prior {
			if word == "--" {
				literal = true
				break
			}
		}
		if !literal && (current == "" || strings.HasPrefix(current, "-")) {
			candidates = append(candidates, "--config", "--home", "--tools", "--json", "--help", "--version")
			if command == "apply" {
				candidates = append(candidates, "--dry-run")
			}
		}
		switch len(opts.positionals) {
		case 0:
			candidates = append(candidates, "list", "tools", "check", "apply", "current", "completion", "help", "version")
		case 1:
			switch command {
			case "completion":
				candidates = append(candidates, "bash", "zsh", "fish")
			case "apply", "check":
				env, err := environment(opts.home)
				if err != nil {
					break
				}
				config, _, err := loadConfig(opts.config, env)
				if err == nil || opts.config == "" {
					candidates = append(candidates, themeIDs(config)...)
				}
			}
		}
		filtered := candidates[:0]
		for _, candidate := range candidates {
			if strings.HasPrefix(candidate, current) {
				filtered = append(filtered, candidate)
			}
		}
		candidates = filtered
	}
	sort.Strings(candidates)
	return candidates
}
