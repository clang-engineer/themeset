package ghostty

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "ghostty", Binary: "ghostty", Resolve: nativeResolveGhostty, Apply: nativeApplyGhostty}
}

func nativeResolveGhostty(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	port, ok, err := shared.ListedCandidates("ghostty", spec.Native, shared.GhosttyThemesSource, env, "+list-themes")
	if ok || err != nil {
		return port, ok, err
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func nativeGhosttyPath(env model.Environment) string {
	xdg := filepath.Join(shared.NativeConfigHome(env), "ghostty", "config")
	if env.OS == "darwin" {
		mac := filepath.Join(env.Home, "Library", "Application Support", "com.mitchellh.ghostty", "config")
		if _, err := os.Stat(mac); err == nil {
			xdg = mac
		}
	}
	return shared.NativePath("ghostty", env, xdg)
}

var nativeGhosttyTheme = regexp.MustCompile(`^\s*theme\s*=`)

func nativeApplyGhostty(port model.Port, env model.Environment) error {
	name := port.Name
	if port.File != "" {
		name = shared.NativeExpand(port.File, env)
		info, err := os.Stat(name)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Ghostty theme must be a regular theme asset: %s", name)
		}
	}
	if name == "" || strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Errorf("invalid Ghostty theme name")
	}
	path := nativeGhosttyPath(env)
	if port.File != "" && filepath.Clean(shared.NativeExpand(port.File, env)) == filepath.Clean(path) {
		return fmt.Errorf("Ghostty theme asset cannot be the destination config")
	}
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	if err := shared.Atomic(path, shared.ReplaceLines(data, func(s string) bool { return nativeGhosttyTheme.MatchString(s) }, "theme = "+strconv.Quote(name))); err != nil {
		return err
	}
	return nativeReloadGhostty(env)
}
