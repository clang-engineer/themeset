package adapter

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// Exact existing upstream assets; the sibling LICENSE files record revisions.
//
//go:embed assets/fzf-dracula.opts
var filesFZFDracula []byte

//go:embed assets/starship-dracula.toml
var filesStarshipDracula []byte

const filesStarshipSource = "https://github.com/dracula/starship/blob/920e9f46ccc25beee15ed7fe0baddabdfeaaf92a/starship.toml"
const filesFZFSource = "https://github.com/dracula/fzf/tree/f4c7758a813ca73b69e9c954c001586c33858c24"
const filesYaziSource = "https://yazi-rs.github.io/docs/flavors/overview/"
const filesAtuinSource = "https://docs.atuin.sh/latest/guide/theming/"
const filesBtopSource = "https://github.com/aristocratos/btop#themes"

func fileAdapters() []model.Adapter {
	return []model.Adapter{
		{ID: "starship", Binary: "starship", Resolve: filesResolveStarship, Apply: filesApplyStarship},
		{ID: "fzf", Binary: "fzf", Resolve: filesResolveFZF, Apply: filesApplyFZF},
		{ID: "yazi", Binary: "yazi", Resolve: filesResolveYazi, Apply: filesApplyYazi},
		{ID: "atuin", Binary: "atuin", Resolve: filesResolveAtuin, Apply: filesApplyAtuin},
		{ID: "lazygit", Binary: "lazygit", Resolve: filesUnsupported, Apply: filesApplyLazy("lazygit")},
		{ID: "lazydocker", Binary: "lazydocker", Resolve: filesUnsupported, Apply: filesApplyLazy("lazydocker")},
		{ID: "btop", Binary: "btop", Resolve: filesResolveBtop, Apply: filesApplyBtop},
		{ID: "eza", Binary: "eza", Resolve: filesUnsupported, Apply: filesApplyEza},
	}
}

func filesResolveStarship(name string, env model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return model.Port{Kind: "official", Name: name, Source: filesStarshipSource}, true, nil
	}
	data, err := filesRead(filesPath("starship", env), true)
	if err != nil {
		return model.Port{}, false, err
	}
	config, err := filesTOML(data)
	if err != nil {
		return model.Port{}, false, err
	}
	palettes, _ := config["palettes"].(map[string]any)
	if palette, ok := palettes[name].(map[string]any); ok && len(palette) > 0 {
		return model.Port{Kind: "native", Name: name, Source: "https://starship.rs/config/#color-palette"}, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyStarship(port model.Port, env model.Environment) error {
	var incoming map[string]any
	if port.File != "" || port.Kind == "official" {
		data := filesStarshipDracula
		if port.File != "" {
			var err error
			data, err = filesCustom(port, env)
			if err != nil {
				return err
			}
		} else if port.Name != "dracula" {
			return errors.New("unknown official Starship port")
		}
		var err error
		incoming, err = filesTOML(data)
		if err != nil {
			return err
		}
		name, _ := incoming["palette"].(string)
		if name == "" {
			name = port.Name
		}
		palettes, _ := incoming["palettes"].(map[string]any)
		palette, ok := palettes[name].(map[string]any)
		if !ok || len(palette) == 0 {
			return errors.New("Starship custom file must define a nonempty selected [palettes.NAME] and palette selector (or port name)")
		}
		port.Name = name
		// Only the selected existing palette is imported, never module settings.
		incoming = map[string]any{name: palette}
	}
	return filesEditTOML(filesPath("starship", env), func(config map[string]any) error {
		palettes, err := filesTable(config, "palettes")
		if err != nil {
			return err
		}
		if incoming != nil {
			filesMerge(palettes, incoming)
		}
		if palette, ok := palettes[port.Name].(map[string]any); !ok || len(palette) == 0 {
			return fmt.Errorf("Starship palette %q is not defined", port.Name)
		}
		config["palette"] = port.Name
		return nil
	})
}

func filesResolveFZF(name string, _ model.Environment) (model.Port, bool, error) {
	if name != "dracula" {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "official", Name: name, Source: filesFZFSource}, true, nil
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

func filesApplyFZF(port model.Port, env model.Environment) error {
	data := filesFZFDracula
	if port.File != "" {
		var err error
		data, err = filesCustom(port, env)
		if err != nil {
			return err
		}
	} else if port.Kind != "official" || port.Name != "dracula" {
		return errors.New("fzf requires an official port or custom options file")
	}
	if err := filesFZFOptions(data); err != nil {
		return err
	}
	path := filesPath("fzf", env)
	existing, err := filesRead(path, true)
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
	return filesAtomic(path, []byte(options.String()))
}

func filesResolveYazi(name string, env model.Environment) (model.Port, bool, error) {
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	root := filepath.Join(filepath.Dir(filesPath("yazi", env)), "flavors", name+".yazi")
	data, err := filesRead(filepath.Join(root, "flavor.toml"), true)
	if err != nil {
		return model.Port{}, false, err
	}
	if len(data) == 0 {
		return model.Port{}, false, nil
	}
	if _, err := filesTOML(data); err != nil {
		return model.Port{}, false, err
	}
	if _, err := filesRead(filepath.Join(root, "tmtheme.xml"), false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Port{}, false, nil
		}
		return model.Port{}, false, err
	}
	return model.Port{Kind: "native", Name: name, File: filepath.Join(root, "flavor.toml"), Source: filesYaziSource}, true, nil
}

func filesApplyYazi(port model.Port, env model.Environment) error {
	var incoming map[string]any
	if port.Kind == "native" {
		if _, ok, err := filesResolveYazi(port.Name, env); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Yazi flavor %q is not installed", port.Name)
		}
		incoming = map[string]any{"flavor": map[string]any{"dark": port.Name, "light": port.Name}}
	} else {
		data, err := filesCustom(port, env)
		if err != nil {
			return err
		}
		incoming, err = filesTOML(data)
		if err != nil {
			return err
		}
		if len(incoming) == 0 {
			return errors.New("Yazi theme file is empty")
		}
	}
	// theme.toml is dedicated to styles. Recursively merge supplied sections,
	// retaining unspecified style overrides; yazi.toml/keymap.toml are untouched.
	return filesEditTOML(filesPath("yazi", env), func(config map[string]any) error {
		filesMerge(config, incoming)
		return nil
	})
}

func filesAtuinThemes(env model.Environment) string {
	candidates := []string{
		filepath.Join(env.Home, ".atuin", "themes"),
		filepath.Join(filepath.Dir(filesPath("atuin", env)), "themes"),
		filepath.Join(filesConfigHome(env), "atuin", "themes"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			return path
		}
	}
	return candidates[1]
}

func filesAtuinBuiltin(name string) bool {
	switch name {
	case "default", "autumn", "marine", "pumpkin", "default-powershell", "(none)":
		return true
	}
	return false
}

func filesAtuinTheme(data []byte) (string, error) {
	config, err := filesTOML(data)
	if err != nil {
		return "", err
	}
	theme, _ := config["theme"].(map[string]any)
	name, _ := theme["name"].(string)
	colors, ok := config["colors"].(map[string]any)
	if !filesName(name) || !ok || len(colors) == 0 {
		return "", errors.New("Atuin theme file requires [theme].name and a nonempty [colors] table")
	}
	for key, value := range colors {
		if text, ok := value.(string); !ok || text == "" {
			return "", fmt.Errorf("Atuin color %s must be a nonempty string", key)
		}
	}
	return name, nil
}

func filesResolveAtuin(name string, env model.Environment) (model.Port, bool, error) {
	if filesAtuinBuiltin(name) {
		return model.Port{Kind: "native", Name: name, Source: filesAtuinSource}, true, nil
	}
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	path := filepath.Join(filesAtuinThemes(env), name+".toml")
	data, err := filesRead(path, true)
	if err != nil {
		return model.Port{}, false, err
	}
	if len(data) == 0 {
		return model.Port{}, false, nil
	}
	actual, err := filesAtuinTheme(data)
	if err != nil {
		return model.Port{}, false, err
	}
	if actual != name {
		return model.Port{}, false, fmt.Errorf("Atuin theme %s declares name %q, expected %q", path, actual, name)
	}
	return model.Port{Kind: "native", Name: name, File: path, Source: filesAtuinSource}, true, nil
}

func filesApplyAtuin(port model.Port, env model.Environment) error {
	var asset []byte
	if port.Kind != "native" {
		var err error
		asset, err = filesCustom(port, env)
		if err != nil {
			return err
		}
		port.Name, err = filesAtuinTheme(asset)
		if err != nil {
			return err
		}
		if filesAtuinBuiltin(port.Name) {
			return errors.New("custom Atuin theme must not shadow a built-in theme name")
		}
	} else if _, ok, err := filesResolveAtuin(port.Name, env); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("Atuin theme %q is not available", port.Name)
	}
	return filesEditTOML(filesPath("atuin", env), func(config map[string]any) error {
		theme, err := filesTable(config, "theme")
		if err != nil {
			return err
		}
		if asset != nil {
			if err := filesAtomic(filepath.Join(filesAtuinThemes(env), port.Name+".toml"), asset); err != nil {
				return err
			}
		}
		theme["name"] = port.Name
		return nil
	})
}

func filesApplyLazy(id string) func(model.Port, model.Environment) error {
	return func(port model.Port, env model.Environment) error {
		data, err := filesCustom(port, env)
		if err != nil {
			return err
		}
		incoming, err := filesYAML(data)
		if err != nil {
			return err
		}
		gui, _ := incoming["gui"].(map[string]any)
		theme, ok := gui["theme"].(map[string]any)
		if !ok || len(theme) == 0 {
			return fmt.Errorf("%s custom file must contain a nonempty gui.theme mapping", id)
		}
		return filesEditYAML(filesPath(id, env), func(config map[string]any) error {
			gui, err := filesTable(config, "gui")
			if err != nil {
				return err
			}
			existing, err := filesTable(gui, "theme")
			if err != nil {
				return err
			}
			filesMerge(existing, theme)
			return nil
		})
	}
}

func filesApplyEza(port model.Port, env model.Environment) error {
	data, err := filesCustom(port, env)
	if err != nil {
		return err
	}
	incoming, err := filesYAML(data)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		return errors.New("eza theme file is empty")
	}
	return filesEditYAML(filesPath("eza", env), func(config map[string]any) error {
		filesMerge(config, incoming)
		return nil
	})
}

func filesBtopDirectories(env model.Environment) []string {
	directories := []string{filepath.Join(filepath.Dir(filesPath("btop", env)), "themes")}
	system := make([]string, 0, 3)
	if binary, err := exec.LookPath("btop"); err == nil {
		if resolved, err := filepath.EvalSymlinks(binary); err == nil {
			binary = resolved
		}
		system = append(system, filepath.Join(filepath.Dir(binary), "..", "share", "btop", "themes"))
	}
	system = append(system, "/usr/local/share/btop/themes", "/usr/share/btop/themes")
	// btop itself uses only the first nonempty system theme directory.
	for _, path := range system {
		entries, err := os.ReadDir(path)
		if err == nil && len(entries) > 0 {
			directories = append(directories, path)
			break
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			directories = append(directories, path)
			break
		}
	}
	return directories
}

var filesBtopThemeLine = regexp.MustCompile(`^\s*theme\[[a-zA-Z0-9_]+\]\s*=\s*"[^"\r\n]*"\s*(#.*)?$`)
var filesBtopSelector = regexp.MustCompile(`^[\t ]*color_theme[\t ]*=`)

func filesBtopTheme(data []byte) error {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !filesBtopThemeLine.MatchString(line) {
			return errors.New("btop custom file must contain native theme[NAME]=\"COLOR\" entries only")
		}
		count++
	}
	if count == 0 {
		return errors.New("btop theme contains no theme entries")
	}
	return nil
}

func filesResolveBtop(name string, env model.Environment) (model.Port, bool, error) {
	if !filesName(name) {
		return model.Port{}, false, nil
	}
	names := []string{name}
	switch name {
	case "gruvbox":
		names = []string{"gruvbox_dark", "gruvbox"}
	case "catppuccin-mocha":
		names = []string{"catppuccin_mocha", "catppuccin-mocha"}
	}
	for _, dir := range filesBtopDirectories(env) {
		for _, candidate := range names {
			path := filepath.Join(dir, candidate+".theme")
			data, err := filesRead(path, true)
			if err != nil {
				return model.Port{}, false, err
			}
			if len(data) == 0 {
				continue
			}
			if err := filesBtopTheme(data); err != nil {
				return model.Port{}, false, fmt.Errorf("parse %s: %w", path, err)
			}
			return model.Port{Kind: "native", Name: candidate, File: path, Source: filesBtopSource}, true, nil
		}
	}
	return model.Port{}, false, nil
}

func filesApplyBtop(port model.Port, env model.Environment) error {
	var asset []byte
	if port.Kind != "native" {
		var err error
		asset, err = filesCustom(port, env)
		if err != nil {
			return err
		}
		if err := filesBtopTheme(asset); err != nil {
			return err
		}
		port.Name = "themeset"
	} else if _, ok, err := filesResolveBtop(port.Name, env); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("btop theme %q is not installed", port.Name)
	}
	path := filesPath("btop", env)
	data, err := filesRead(path, true)
	if err != nil {
		return err
	}
	setting := fmt.Sprintf("color_theme = %q", port.Name)
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if filesBtopSelector.MatchString(line) {
			lines[i] = setting
			found = true
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		lines = append(lines, setting, "")
	}
	if asset != nil {
		if err := filesAtomic(filepath.Join(filepath.Dir(path), "themes", "themeset.theme"), asset); err != nil {
			return err
		}
	}
	return filesAtomic(path, []byte(strings.Join(lines, "\n")))
}
