package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

func nativeGhosttyPath(env model.Environment) string {
	xdg := filepath.Join(nativeConfigHome(env), "ghostty", "config")
	if env.OS == "darwin" {
		mac := filepath.Join(env.Home, "Library", "Application Support", "com.mitchellh.ghostty", "config")
		if _, err := os.Stat(mac); err == nil {
			xdg = mac
		}
	}
	return nativePath("ghostty", env, xdg)
}

var nativeGhosttyTheme = regexp.MustCompile(`^\s*theme\s*=`)

func nativeApplyGhostty(port model.Port, env model.Environment) error {
	name := port.Name
	if port.File != "" {
		name = nativeExpand(port.File, env)
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
	if port.File != "" && filepath.Clean(nativeExpand(port.File, env)) == filepath.Clean(path) {
		return fmt.Errorf("Ghostty theme asset cannot be the destination config")
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	if err := nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return nativeGhosttyTheme.MatchString(s) }, "theme = "+strconv.Quote(name))); err != nil {
		return err
	}
	return nativeReloadGhostty(env)
}
