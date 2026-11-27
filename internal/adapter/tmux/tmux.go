package tmux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "tmux", Binary: "tmux", Resolve: nativeResolveTmux, Apply: nativeApplyTmux}
}

// Registered upstream plugins own their full layout. Missing plugins are
// planned without network access and installed only when apply is confirmed.
type pluginSpec struct {
	repo, entry, option, variant string
	required                     []string
	minVersion                   bool
}

func tmuxPlugin(name string) (pluginSpec, bool) {
	switch name {
	case "catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha":
		flavor := strings.TrimPrefix(name, "catppuccin-")
		return pluginSpec{"catppuccin/tmux", "catppuccin.tmux", "@catppuccin_flavor", flavor,
			[]string{"catppuccin_options_tmux.conf", "catppuccin_tmux.conf", "themes/catppuccin_" + flavor + "_tmux.conf"}, true}, true
	case "rose-pine", "rose-pine-moon", "rose-pine-dawn":
		variant := strings.TrimPrefix(name, "rose-pine-")
		if variant == "rose-pine" {
			variant = "main"
		}
		return pluginSpec{"rose-pine/tmux", "rose-pine.tmux", "@rose_pine_variant", variant, nil, false}, true
	case "dracula":
		return pluginSpec{"dracula/tmux", "dracula.tmux", "", "", []string{"scripts/dracula.sh"}, false}, true
	case "nord":
		return pluginSpec{"nordtheme/tmux", "nord.tmux", "", "",
			[]string{"src/nord.conf", "src/nord-status-content.conf", "src/nord-status-content-no-patched-font.conf"}, false}, true
	case "gruvbox":
		return pluginSpec{"egel/tmux-gruvbox", "gruvbox-tpm.tmux", "@tmux-gruvbox", "dark256",
			[]string{"src/gruvbox-main.sh", "src/helper_methods.sh", "src/tmux_utils.sh",
				"src/palette_gruvbox_dark256.sh", "src/theme_gruvbox_dark.sh"}, false}, true
	case "everforest":
		return pluginSpec{"TanglingTreats/tmux-everforest", "everforest-theme.tmux", "@tmux-everforest", "dark-medium",
			[]string{"tmux-everforest-dark-medium.conf"}, false}, true
	}
	return pluginSpec{}, false
}

func pluginInstalled(entrypoint string, plugin pluginSpec) bool {
	root := filepath.Dir(entrypoint)
	info, err := os.Stat(entrypoint)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	for _, path := range plugin.required {
		info, err := os.Stat(filepath.Join(root, path))
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
	}
	return true
}

func nativeResolveTmux(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	for _, name := range spec.Native {
		plugin, registered := tmuxPlugin(name)
		if !registered {
			continue
		}
		paths := []string{
			filepath.Join(shared.NativeConfigHome(env), "tmux", "plugins", plugin.repo, plugin.entry),
			filepath.Join(env.Home, ".tmux", "plugins", filepath.Base(plugin.repo), plugin.entry),
		}
		path := paths[0]
		for _, candidate := range paths {
			if pluginInstalled(candidate, plugin) {
				path = candidate
				break
			}
		}
		return model.Port{Kind: "native", Name: name, File: path, Source: "https://github.com/" + plugin.repo}, true, nil
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func checkPluginVersion(name string, plugin pluginSpec, env model.Environment) error {
	if !plugin.minVersion {
		return nil
	}
	version, err := shared.Command(env, "tmux", "-V")
	if err != nil {
		return err
	}
	var major, minor int
	if n, err := fmt.Sscanf(strings.TrimSpace(string(version)), "tmux %d.%d", &major, &minor); n != 2 || err != nil || major < 3 || major == 3 && minor < 2 {
		return fmt.Errorf("%s requires tmux 3.2 or newer (found %q)", name, strings.TrimSpace(string(version)))
	}
	return nil
}

// Install into a private staging directory first; a failed clone or an
// incomplete repository must not replace an existing installation or config.
func installPlugin(entrypoint string, plugin pluginSpec, env model.Environment) error {
	target := filepath.Dir(entrypoint)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("incomplete tmux plugin at %s; repair it manually", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(target), ".themeset-plugin-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	args := []string{"clone", "--quiet", "--depth", "1"}
	if plugin.repo == "catppuccin/tmux" {
		args = append(args, "--branch", "v2.3.2")
	}
	args = append(args, "https://github.com/"+plugin.repo+".git", stage)
	if _, err := shared.Command(env, "git", args...); err != nil {
		return fmt.Errorf("install tmux plugin %s: %w", plugin.repo, err)
	}
	if !pluginInstalled(filepath.Join(stage, plugin.entry), plugin) {
		return fmt.Errorf("installed tmux plugin %s lacks required files", plugin.repo)
	}
	if err := os.Rename(stage, target); err != nil {
		return fmt.Errorf("activate tmux plugin %s: %w", plugin.repo, err)
	}
	return nil
}

func nativeApplyTmux(port model.Port, env model.Environment) error {
	official := port.Kind == "official" && catalog.Verified("tmux", port)
	if port.Kind == "official" && !official {
		return fmt.Errorf("unregistered official tmux port %q", port.Name)
	}
	if port.File == "" && !official {
		return fmt.Errorf("tmux requires an official port or a registered tmux theme config file")
	}
	source := shared.NativeExpand(port.File, env)
	plugin, registered := tmuxPlugin(port.Name)
	installed := port.Kind == "native" && registered
	if installed {
		source = filepath.Join(shared.NativeConfigHome(env), "themeset", "tmux-"+port.Name+".conf")
	}
	if official {
		source = filepath.Join(shared.NativeConfigHome(env), "themeset", "tmux-"+port.Name+".conf")
	}
	if !filepath.IsAbs(source) || strings.ContainsAny(source, "\r\n\x00") {
		return fmt.Errorf("tmux theme config requires an absolute file path")
	}
	if !official && !installed {
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("tmux theme config must be a regular file")
		}
	}
	fallback := filepath.Join(env.Home, ".tmux.conf")
	xdg := filepath.Join(shared.NativeConfigHome(env), "tmux", "tmux.conf")
	if _, err := os.Stat(fallback); os.IsNotExist(err) {
		if _, err := os.Stat(xdg); err == nil {
			fallback = xdg
		}
	}
	path := shared.NativePath("tmux", env, fallback)
	if filepath.Clean(path) == filepath.Clean(source) {
		return fmt.Errorf("tmux theme source cannot be the destination config")
	}
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	if installed {
		entrypoint := shared.NativeExpand(port.File, env)
		if err := checkPluginVersion(port.Name, plugin, env); err != nil {
			return err
		}
		if !pluginInstalled(entrypoint, plugin) {
			if err := installPlugin(entrypoint, plugin, env); err != nil {
				return err
			}
		}
	}
	if installed {
		entrypoint := shared.NativeExpand(port.File, env)
		// Quote once for the shell and once for tmux's config parser.
		shellPath := "'" + strings.ReplaceAll(entrypoint, "'", "'\\''") + "'"
		content := ""
		if plugin.option != "" {
			content = fmt.Sprintf("set -g %s %q\n", plugin.option, plugin.variant)
		}
		content += "run-shell " + strconv.Quote(shellPath) + "\n"
		if port.Name == "catppuccin-latte" || port.Name == "catppuccin-frappe" || port.Name == "catppuccin-macchiato" || port.Name == "catppuccin-mocha" {
			content += "set -g status-left \"#{E:@catppuccin_status_session}\"\nset -g status-right \"#{E:@catppuccin_status_application}\"\n"
		}
		if err := shared.Atomic(source, []byte(content)); err != nil {
			return err
		}
	} else if official {
		data, err := shared.AcquireAsset(env, port.Source)
		if err != nil {
			return fmt.Errorf("acquire tmux styles: %w", err)
		}
		styles, err := filesTmuxStyles(data, port)
		if err != nil {
			return err
		}
		if err := shared.Atomic(source, styles); err != nil {
			return err
		}
	}
	// Custom sources are explicitly trusted tmux code; official sources contain
	// only styles, while installed upstream plugins control the entire layout.
	quoted := "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`").Replace(source) + "\""
	if err := shared.Atomic(path, shared.ReplaceLines(data, func(s string) bool { return strings.HasSuffix(s, " # themeset") }, "source-file "+quoted+" # themeset")); err != nil {
		return err
	}
	return nativeReloadTmux(path, fallback, env)
}

// Import only literal color-style commands selected by the bundled declaration.
// Never execute an upstream theme script or let it replace the user's layout.
var filesTmuxStyle = regexp.MustCompile(`^(set|setw) -g ([a-z-]+) ("[a-zA-Z0-9,#=]+"|none|NONE,[a-zA-Z0-9,#=]+)$`)

func filesTmuxStyles(data []byte, port model.Port) ([]byte, error) {
	allowed := strings.Fields(port.StyleKeys)
	required := strings.Fields(port.RequiredStyleKeys)
	if len(allowed) == 0 || len(required) == 0 {
		return nil, errors.New("tmux style declaration requires allowed and required keys")
	}
	seen := make([]bool, len(required))
	var result strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		match := filesTmuxStyle.FindStringSubmatch(line)
		if len(match) == 0 || !slices.Contains(allowed, match[2]) {
			continue
		}
		result.WriteString(line)
		result.WriteByte('\n')
		for i, key := range required {
			if match[2] == key {
				seen[i] = true
			}
		}
	}
	for i, ok := range seen {
		if !ok {
			return nil, fmt.Errorf("tmux source has no %s style", required[i])
		}
	}
	return []byte(result.String()), nil
}
