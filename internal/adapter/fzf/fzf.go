package fzf

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "fzf", Binary: "fzf", Remote: true, Resolve: filesResolveFZF, Apply: filesApplyFZF}
}

func filesResolveFZF(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	if spec.Official != nil {
		if spec.Official.Format == "fzf-rc" {
			if err := filesFZFRequireSelectedColors(env); err != nil {
				return model.Port{}, false, err
			}
		}
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
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

func filesFZFColorsAtLeast(version []byte, minorVersion int) bool {
	text := strings.TrimSpace(string(version))
	if !filesFZFVersion.MatchString(text) {
		return false
	}
	var major, minor, patch int
	if n, err := fmt.Sscanf(text, "%d.%d.%d", &major, &minor, &patch); err != nil || n != 3 {
		return false
	}
	return major > 0 || minor >= minorVersion
}

func filesFZFSelectedColors(version []byte) bool {
	return filesFZFColorsAtLeast(version, 52)
}

func filesFZFRequireSelectedColors(env model.Environment) error {
	version, err := shared.Command(env, "fzf", "--version")
	if err != nil {
		return fmt.Errorf("fzf selected color styles require version 0.52.0 or newer: %w", err)
	}
	if !filesFZFSelectedColors(version) {
		return errors.New("fzf selected color styles require version 0.52.0 or newer")
	}
	return nil
}

func filesFZFRequireFooterColors(env model.Environment) error {
	version, err := shared.Command(env, "fzf", "--version")
	if err != nil {
		return fmt.Errorf("fzf footer color styles require version 0.63.0 or newer: %w", err)
	}
	if !filesFZFColorsAtLeast(version, 63) {
		return errors.New("fzf footer color styles require version 0.63.0 or newer")
	}
	return nil
}

func filesApplyFZF(port model.Port, env model.Environment) error {
	var data []byte
	if port.File != "" || port.URL != "" {
		var err error
		data, err = shared.Custom(port, env)
		if err != nil {
			return err
		}
	} else if port.Kind == "official" {
		if !catalog.Verified("fzf", port) {
			return errors.New("unknown official fzf port")
		}
		if port.Format == "fzf-rc" {
			if err := filesFZFRequireSelectedColors(env); err != nil {
				return err
			}
		}
		var err error
		data, err = shared.AcquireAsset(env, port.Source)
		if err != nil {
			return fmt.Errorf("acquire %s fzf theme: %w", port.Name, err)
		}
		switch port.Format {
		case "fzf-script-colors":
			data, err = filesExtractFZFScriptColors(data)
		case "fzf-export-colors":
			data, err = filesExtractFZFExportColors(data)
		default:
			data, err = filesPrepareFZFOfficial(port.Format, port.Variant, data)
		}
		if err != nil {
			return fmt.Errorf("invalid %s fzf theme: %w", port.Name, err)
		}
		if port.Format == "fzf-presets" && bytes.Contains(data, []byte("--color=footer:")) {
			if err := filesFZFRequireFooterColors(env); err != nil {
				return err
			}
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
func filesExtractFZFScriptColors(data []byte) ([]byte, error) {
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
func filesExtractFZFExportColors(script []byte) ([]byte, error) {
	lines := strings.Split(strings.TrimSpace(string(script)), "\n")
	if len(lines) < 2 || lines[0] != `export FZF_DEFAULT_OPTS="` || !strings.HasSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`) {
		return nil, errors.New("unexpected fzf color export wrapper")
	}
	lines[len(lines)-1] = strings.TrimSuffix(strings.TrimSpace(lines[len(lines)-1]), `"`)
	var result strings.Builder
	for _, line := range lines[1:] {
		option := strings.TrimSpace(line)
		if !strings.HasPrefix(option, "--color=") {
			return nil, fmt.Errorf("unexpected fzf color export option %q", option)
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

var filesFZFColor = regexp.MustCompile(`^([a-z][a-z+-]*):#[[:xdigit:]]{6}$`)
var filesFZFPresetEntry = regexp.MustCompile(`^\s*('?[-a-z]+'?):'(#[[:xdigit:]]{6})',?\s*$`)

// Validate the extracted official colors, not just their fzf option syntax:
// an upstream page or shell script must not smuggle extra options or commands.
func filesFZFOfficialColors(format string, data []byte) ([]byte, error) {
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
			case "selected-bg":
				if format != "fzf-rc" {
					return nil, fmt.Errorf("unsupported fzf color %q", color)
				}
			case "label":
				if format != "fzf-rc" && format != "fzf-presets" {
					return nil, fmt.Errorf("unsupported fzf color %q", color)
				}
			case "footer":
				if format != "fzf-presets" {
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

func filesPrepareFZFOfficial(format, variant string, source []byte) ([]byte, error) {
	switch format {
	case "fzf-rc":
		return filesFZFOfficialColors(format, source)
	case "fzf-install":
		// Extract only options embedded in the upstream shell recipe.
		const prefix = "export FZF_DEFAULT_OPTS='"
		for _, line := range strings.Split(string(source), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "'") {
				continue
			}
			options := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "'")
			return filesFZFOfficialColors(format, []byte(strings.Join(strings.Fields(options), "\n")+"\n"))
		}
		return nil, errors.New("fzf shell recipe missing")
	case "fzf-shell":
		// Only literal --color pairs in the one upstream export are imported.
		var colors strings.Builder
		started, closed := false, false
		for _, raw := range strings.Split(string(source), "\n") {
			line := strings.TrimSpace(raw)
			if !started {
				if strings.HasPrefix(line, "export FZF_DEFAULT_OPTS=") {
					if line != `export FZF_DEFAULT_OPTS="$FZF_DEFAULT_OPTS \` {
						return nil, errors.New("unexpected fzf color export")
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
				return nil, fmt.Errorf("unexpected fzf export color %q", line)
			}
			colors.WriteString("--color=")
			colors.WriteString(fields[1])
			colors.WriteByte('\n')
		}
		if !started || !closed {
			return nil, errors.New("incomplete fzf color export")
		}
		return filesFZFOfficialColors(format, []byte(colors.String()))
	case "fzf-presets":
		// Parse only the literal palette object; never execute upstream JS.
		marker := []byte("  '" + variant + "': {")
		start := bytes.Index(source, marker)
		if start < 0 || start > 0 && source[start-1] != '\n' {
			return nil, fmt.Errorf("fzf preset %q missing", variant)
		}
		body := source[start+len(marker):]
		end := bytes.Index(body, []byte("\n  },"))
		if end < 0 {
			return nil, fmt.Errorf("fzf preset %q not terminated", variant)
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
		data, err := filesFZFOfficialColors(format, []byte(colors.String()))
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"fg", "bg", "hl", "fg+", "bg+", "hl+", "prompt", "marker", "border"} {
			if !strings.Contains(string(data), "--color="+key+":") {
				return nil, fmt.Errorf("fzf preset %q missing %s", variant, key)
			}
		}
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported official fzf format %q", format)
	}
}
