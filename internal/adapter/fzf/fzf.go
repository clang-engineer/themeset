package fzf

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "fzf", Binary: "fzf", Resolve: filesResolveFZF, Apply: filesApplyFZF}
}

func filesResolveFZF(name string, env model.Environment) (model.Port, bool, error) {
	if port, found, err := shared.ResolveTokyoNightPort("fzf", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := shared.ResolveRosePinePort("fzf", name, env); found || err != nil {
		return port, found, err
	}
	switch name {
	case "catppuccin-mocha":
		if err := filesFZFRequireSelectedColors(env); err != nil {
			return model.Port{}, false, err
		}
		return model.Port{Kind: "official", Name: name, Source: shared.FZFCatppuccinSource}, true, nil
	case "gruvbox", "nord":
		return model.Port{Kind: "official", Name: name, Source: shared.FZFPresetsSource}, true, nil
	case "dracula":
		return model.Port{Kind: "official", Name: name, Source: shared.FZFDraculaSource}, true, nil
	case "moonfly":
		return model.Port{Kind: "official", Name: name, Source: shared.MoonflySource + "moonfly-fzf.sh"}, true, nil
	default:
		return model.Port{}, false, nil
	}
}

// Custom fzf ports contain one --color=... option per line, not shell code or
// executable --bind/--preview options. This file is appended by shell integration
// rather than replacing the user's ordinary FZF_DEFAULT_OPTS(_FILE).
func filesFZFOptions(data []byte) error {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "--color=") || len(line) == len("--color=") || strings.ContainsAny(line, " \t\r\x00\"'`$") {
			return errors.New("fzf theme files require one unquoted --color=... option per line; other options and shell code are not accepted")
		}
		count++
	}
	if count == 0 {
		return errors.New("fzf theme file contains no --color options")
	}
	return nil
}

var filesFZFVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:\s|$)`)

func filesFZFSelectedColors(version []byte) bool {
	text := strings.TrimSpace(string(version))
	if !filesFZFVersion.MatchString(text) {
		return false
	}
	var major, minor, patch int
	if n, err := fmt.Sscanf(text, "%d.%d.%d", &major, &minor, &patch); err != nil || n != 3 {
		return false
	}
	return major > 0 || minor >= 52
}

func filesFZFRequireSelectedColors(env model.Environment) error {
	version, err := shared.Command(env, "fzf", "--version")
	if err != nil {
		return fmt.Errorf("Catppuccin Mocha requires fzf 0.52.0 or newer: %w", err)
	}
	if !filesFZFSelectedColors(version) {
		return errors.New("Catppuccin Mocha requires fzf 0.52.0 or newer")
	}
	return nil
}

func filesApplyFZF(port model.Port, env model.Environment) error {
	var data []byte
	if port.File != "" {
		var err error
		data, err = shared.Custom(port, env)
		if err != nil {
			return err
		}
	} else if port.Kind == "official" {
		switch port.Name {
		case "catppuccin-mocha":
			if err := filesFZFRequireSelectedColors(env); err != nil {
				return err
			}
			fallthrough
		case "gruvbox", "nord", "dracula", "moonfly":
			source := filesFZFOfficialSource(port.Name)
			if port.Source != source {
				return fmt.Errorf("unregistered official fzf source for %q", port.Name)
			}
			var err error
			data, err = shared.AcquireAsset(env, source)
			if err != nil {
				return fmt.Errorf("acquire %s fzf theme: %w", port.Name, err)
			}
			data, err = filesPrepareFZFOfficial(port.Name, data)
			if err != nil {
				return fmt.Errorf("invalid %s fzf theme: %w", port.Name, err)
			}
		case "tokyo-night":
			var err error
			data, err = filesAcquireTokyoNightPort(port, env)
			if err != nil {
				return err
			}
		case "rose-pine":
			var err error
			data, err = filesAcquireRosePinePort(port, env)
			if err != nil {
				return err
			}
		default:
			return errors.New("unknown official fzf port")
		}
	} else {
		return errors.New("fzf requires an official port or custom options file")
	}
	if err := filesFZFOptions(data); err != nil {
		return err
	}
	path := shared.Path("fzf", env)
	existing, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if err := filesFZFOptions(existing); err != nil {
			return fmt.Errorf("refusing to replace non-theme options in %s: %w", path, err)
		}
	}
	var options strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			options.WriteString(line)
			options.WriteByte('\n')
		}
	}
	return shared.Atomic(path, []byte(options.String()))
}

// The author ships a shell script setting layout, preview and ANSI options.
// Extract only its literal --color entries; executing or importing shell code
// would overwrite the user's unrelated fzf configuration.
func filesTokyoNightFZF(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var colors strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), `\`))
		if !strings.HasPrefix(line, "--color=") {
			continue
		}
		colors.WriteString(line)
		colors.WriteByte('\n')
	}
	result := []byte(colors.String())
	if err := filesFZFOptions(result); err != nil {
		return nil, err
	}
	return result, nil
}

// Discard the shell export, retaining only the author's exact fzf color options.
func rosePineFZFOptions(script []byte) ([]byte, error) {
	lines := strings.Split(strings.TrimSpace(string(script)), "\n")
	if len(lines) < 2 || lines[0] != `export FZF_DEFAULT_OPTS="` || !strings.HasSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`) {
		return nil, errors.New("unexpected Rosé Pine fzf shell wrapper")
	}
	lines[len(lines)-1] = strings.TrimSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`)
	var result strings.Builder
	for _, line := range lines[1:] {
		option := strings.TrimSpace(line)
		if !strings.HasPrefix(option, "--color=") {
			return nil, fmt.Errorf("unexpected Rosé Pine fzf option %q", option)
		}
		result.WriteString(option)
		result.WriteByte('\n')
	}
	data := []byte(result.String())
	if err := filesFZFOptions(data); err != nil {
		return nil, err
	}
	return data, nil
}

func filesPrepareFZFPort(theme string, data []byte) ([]byte, error) {
	switch theme {
	case "rose-pine":
		return rosePineFZFOptions(data)
	case "tokyo-night":
		return filesTokyoNightFZF(data)
	default:
		return nil, fmt.Errorf("unsupported fzf port %q", theme)
	}
}

func filesAcquireRosePinePort(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireRosePinePort("fzf", port, env)
	if err != nil {
		return nil, err
	}
	data, err = filesPrepareFZFPort("rose-pine", data)
	if err != nil {
		return nil, fmt.Errorf("invalid Rosé Pine fzf theme: %w", err)
	}
	return data, nil
}

func filesAcquireTokyoNightPort(port model.Port, env model.Environment) ([]byte, error) {
	data, err := shared.AcquireTokyoNightPort("fzf", port, env)
	if err != nil {
		return nil, err
	}
	data, err = filesPrepareFZFPort("tokyo-night", data)
	if err != nil {
		return nil, fmt.Errorf("invalid Tokyo Night fzf port: %w", err)
	}
	return data, nil
}

func filesFZFOfficialSource(name string) string {
	switch name {
	case "catppuccin-mocha":
		return shared.FZFCatppuccinSource
	case "gruvbox", "nord":
		return shared.FZFPresetsSource
	case "dracula":
		return shared.FZFDraculaSource
	case "moonfly":
		return shared.MoonflySource + "moonfly-fzf.sh"
	}
	return ""
}

var filesFZFColor = regexp.MustCompile(`^([a-z][a-z+-]*):#[[:xdigit:]]{6}$`)
var filesFZFPresetEntry = regexp.MustCompile(`^\s*('?[-a-z]+'?):'(#[[:xdigit:]]{6})',?\s*$`)

// Validate the extracted official colors, not just their fzf option syntax:
// an upstream page or shell script must not smuggle extra options or commands.
func filesFZFOfficialColors(theme string, data []byte) ([]byte, error) {
	if err := filesFZFOptions(data); err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	for _, line := range strings.Fields(string(data)) {
		for _, color := range strings.Split(strings.TrimPrefix(line, "--color="), ",") {
			match := filesFZFColor.FindStringSubmatch(color)
			if match == nil || seen[match[1]] {
				return nil, fmt.Errorf("invalid or repeated fzf color %q", color)
			}
			switch match[1] {
			case "fg", "bg", "hl", "fg+", "bg+", "hl+", "info", "prompt", "pointer", "marker", "spinner", "header", "border", "gutter":
			case "selected-bg", "label":
				if theme != "catppuccin-mocha" {
					return nil, fmt.Errorf("unsupported fzf color %q", color)
				}
			default:
				return nil, fmt.Errorf("unsupported fzf color %q", color)
			}
			seen[match[1]] = true
		}
	}
	return data, nil
}

func filesPrepareFZFOfficial(theme string, source []byte) ([]byte, error) {
	switch theme {
	case "catppuccin-mocha":
		return filesFZFOfficialColors(theme, source)
	case "dracula":
		// Dracula publishes its fzf options inside a fenced shell recipe.
		const prefix = "export FZF_DEFAULT_OPTS='"
		for _, line := range strings.Split(string(source), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "'") {
				continue
			}
			options := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "'")
			return filesFZFOfficialColors(theme, []byte(strings.Join(strings.Fields(options), "\n")+"\n"))
		}
		return nil, errors.New("Dracula fzf shell recipe missing")
	case "moonfly":
		// Only literal --color pairs in the one upstream export are imported.
		var colors strings.Builder
		started, closed := false, false
		for _, raw := range strings.Split(string(source), "\n") {
			line := strings.TrimSpace(raw)
			if !started {
				if strings.HasPrefix(line, "export FZF_DEFAULT_OPTS=") {
					if line != `export FZF_DEFAULT_OPTS="$FZF_DEFAULT_OPTS \` {
						return nil, errors.New("unexpected Moonfly fzf export")
					}
					started = true
				}
				continue
			}
			if line == `"` {
				closed = true
				break
			}
			fields := strings.Fields(strings.TrimSpace(strings.TrimSuffix(line, `\`)))
			if len(fields) != 2 || fields[0] != "--color" || !strings.HasSuffix(line, `\`) && !strings.HasPrefix(fields[1], "spinner:") {
				return nil, fmt.Errorf("unexpected Moonfly fzf color %q", line)
			}
			colors.WriteString("--color=")
			colors.WriteString(fields[1])
			colors.WriteByte('\n')
		}
		if !started || !closed {
			return nil, errors.New("incomplete Moonfly fzf export")
		}
		return filesFZFOfficialColors(theme, []byte(colors.String()))
	case "gruvbox", "nord":
		preset := "Gruvbox Dark"
		if theme == "nord" {
			preset = "Nord"
		}
		// Parse only the literal palette object; never execute upstream JS.
		marker := []byte("  '" + preset + "': {")
		start := bytes.Index(source, marker)
		if start < 0 || start > 0 && source[start-1] != '\n' {
			return nil, fmt.Errorf("fzf preset %q missing", preset)
		}
		body := source[start+len(marker):]
		end := bytes.Index(body, []byte("\n  },"))
		if end < 0 {
			return nil, fmt.Errorf("fzf preset %q not terminated", preset)
		}
		var colors strings.Builder
		for _, line := range strings.Split(string(body[:end]), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			for _, entry := range strings.Split(line, ",") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				match := filesFZFPresetEntry.FindStringSubmatch(entry)
				if match == nil {
					return nil, fmt.Errorf("invalid fzf preset entry %q", entry)
				}
				key := strings.Trim(match[1], "'")
				switch key {
				case "current-fg":
					key = "fg+"
				case "current-bg":
					key = "bg+"
				case "current-hl":
					key = "hl+"
				}
				colors.WriteString("--color=")
				colors.WriteString(key)
				colors.WriteByte(':')
				colors.WriteString(match[2])
				colors.WriteByte('\n')
			}
		}
		data, err := filesFZFOfficialColors(theme, []byte(colors.String()))
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"fg", "bg", "hl", "fg+", "bg+", "hl+", "prompt", "marker", "border"} {
			if !strings.Contains(string(data), "--color="+key+":") {
				return nil, fmt.Errorf("fzf preset %q missing %s", preset, key)
			}
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported official fzf port %q", theme)
	}
}
