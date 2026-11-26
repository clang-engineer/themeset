package adapter

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

// Only exact upstream style commands, never status layouts or bindings.
// See assets/moonfly.LICENSE for the pinned source and omitted commands.
//
//go:embed assets/tmux-moonfly.conf
var nativeTmuxMoonfly []byte

func nativeResolveTmux(name string, _ model.Environment) (model.Port, bool, error) {
	switch name {
	case "moonfly":
		return model.Port{Kind: "official", Name: name, Source: filesMoonflySource + "moonfly.tmux"}, true, nil
	case "tokyo-night":
		return model.Port{Kind: "official", Name: name, Source: filesTokyoNightBase + filesTokyoNightAssets["tmux"].path}, true, nil
	}
	return model.Port{}, false, nil
}

func nativeApplyTmux(port model.Port, env model.Environment) error {
	moonfly := port.File == "" && port.Kind == "official" && port.Name == "moonfly" && port.Source == filesMoonflySource+"moonfly.tmux"
	tokyo := port.File == "" && port.Kind == "official" && port.Name == "tokyo-night" && port.Source == filesTokyoNightBase+filesTokyoNightAssets["tmux"].path
	if port.File == "" && !moonfly && !tokyo {
		return fmt.Errorf("tmux requires an official port or a registered tmux theme config file")
	}
	source := nativeExpand(port.File, env)
	if moonfly || tokyo {
		source = filepath.Join(nativeConfigHome(env), "themeset", "tmux-"+port.Name+".conf")
	}
	if !filepath.IsAbs(source) || strings.ContainsAny(source, "\r\n\x00") {
		return fmt.Errorf("tmux theme config requires an absolute file path")
	}
	if !moonfly && !tokyo {
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("tmux theme config must be a regular file")
		}
	}
	fallback := filepath.Join(env.Home, ".tmux.conf")
	xdg := filepath.Join(nativeConfigHome(env), "tmux", "tmux.conf")
	if _, err := os.Stat(fallback); os.IsNotExist(err) {
		if _, err := os.Stat(xdg); err == nil {
			fallback = xdg
		}
	}
	path := nativePath("tmux", env, fallback)
	if filepath.Clean(path) == filepath.Clean(source) {
		return fmt.Errorf("tmux theme source cannot be the destination config")
	}
	data, err := nativeRead(path)
	if err != nil {
		return err
	}
	if moonfly {
		if err := nativeWrite(source, nativeTmuxMoonfly); err != nil {
			return err
		}
	} else if tokyo {
		styles, err := nativeAcquireTokyoNightTmux(env)
		if err != nil {
			return err
		}
		if err := nativeWrite(source, styles); err != nil {
			return err
		}
	}
	// Custom sources are explicitly trusted tmux code; bundled sources include
	// only style commands. No shell or live server is run; the next config load
	// includes the theme after existing configuration.
	quoted := "\"" + strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`").Replace(source) + "\""
	return nativeWrite(path, nativeReplaceLines(data, func(s string) bool { return strings.HasSuffix(s, " # themeset") }, "source-file "+quoted+" # themeset"))
}
