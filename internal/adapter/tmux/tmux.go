package tmux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "tmux", Binary: "tmux", Resolve: nativeResolveTmux, Apply: nativeApplyTmux}
}

// Only style commands from the current author source are imported; full tmux
// theme scripts can overwrite the user's status layout or run shell commands.

func nativeResolveTmux(spec model.ThemePort, _ model.Environment) (model.Port, bool, error) {
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
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
	if official {
		source = filepath.Join(shared.NativeConfigHome(env), "themeset", "tmux-"+port.Name+".conf")
	}
	if !filepath.IsAbs(source) || strings.ContainsAny(source, "\r\n\x00") {
		return fmt.Errorf("tmux theme config requires an absolute file path")
	}
	if !official {
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
	if official {
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
	// Custom sources are explicitly trusted tmux code; official sources include
	// only style commands. No shell or live server is run; the next config load
	// includes the theme after existing configuration.
	quoted := "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`").Replace(source) + "\""
	return shared.Atomic(path, shared.ReplaceLines(data, func(s string) bool { return strings.HasSuffix(s, " # themeset") }, "source-file "+quoted+" # themeset"))
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
