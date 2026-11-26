package delta

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "delta", Binary: "delta", Resolve: nativeResolveDelta, Apply: nativeApplyDelta}
}

func nativeDeltaThemeKey(key string) bool {
	if !strings.HasPrefix(strings.ToLower(key), "delta.") {
		return false
	}
	_, option, _ := strings.Cut(key, ".")
	if index := strings.LastIndexByte(option, '.'); index >= 0 {
		option = option[index+1:]
	}
	option = strings.ToLower(option)
	return option == "syntax-theme" || option == "features" || option == "dark" || option == "light" || strings.HasSuffix(option, "-style") || strings.HasSuffix(option, "-syntax-theme")
}

func nativeResolveDelta(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	port, ok, err := shared.ListedCandidates("delta", spec.Native, shared.DeltaThemesSource, env, "--list-syntax-themes")
	if ok || err != nil {
		return port, ok, err
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

// Import only the selected Delta feature. Upstream assets can contain other
// variants and default dark/light selectors; none belong in global Git config.
func nativeDeltaFeature(data []byte, feature string) ([]byte, error) {
	text := string(data)
	header := "[delta \"" + feature + "\"]"
	first, rest, ok := strings.Cut(text, "\n")
	if !ok || first != header {
		return nil, fmt.Errorf("Delta feature %q missing from official asset", feature)
	}
	end := len(rest)
	for start := 0; start < len(rest); {
		lineEnd := strings.IndexByte(rest[start:], '\n')
		if lineEnd < 0 {
			lineEnd = len(rest)
		} else {
			lineEnd += start
		}
		if strings.HasPrefix(strings.TrimLeft(rest[start:lineEnd], " \t"), "[") {
			end = start
			break
		}
		start = lineEnd + 1
	}
	main := header + "\n" + rest[:end]
	if !strings.Contains(main, "\n\tfile-style = ") {
		return nil, fmt.Errorf("Delta feature %q has no file style", feature)
	}
	return []byte(strings.TrimRight(main, "\n") + "\n"), nil
}

func nativeApplyOfficialDeltaFeature(port model.Port, env model.Environment) error {
	data, err := shared.AcquireAsset(env, port.Source)
	if err != nil {
		return err
	}
	main, err := nativeDeltaFeature(data, port.Name)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(env.Home, ".themeset-delta-feature-*.gitconfig")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(main); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return nativeApplyDelta(model.Port{Kind: "feature", Name: port.Name, File: file.Name()}, env)
}

func nativeApplyDelta(port model.Port, env model.Environment) error {
	if port.Kind == "official" {
		if !catalog.Verified("delta", port) {
			return fmt.Errorf("unregistered official Delta port %q", port.Name)
		}
		if port.Format != "delta-feature" {
			return fmt.Errorf("unsupported official Delta format %q", port.Format)
		}
		return nativeApplyOfficialDeltaFeature(port, env)
	}
	settings := map[string]string{}
	if port.File != "" {
		source := shared.NativeExpand(port.File, env)
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("delta custom config must be a regular gitconfig file")
		}
		out, err := shared.Command(env, "git", "config", "--file", source, "--no-includes", "--null", "--list")
		if err != nil {
			return err
		}
		for _, entry := range strings.Split(string(out), "\x00") {
			key, value, ok := strings.Cut(entry, "\n")
			if ok && nativeDeltaThemeKey(key) {
				settings[key] = value
			}
		}
	}
	if port.Name != "" {
		if strings.ContainsAny(port.Name, "\r\n\x00") {
			return fmt.Errorf("invalid delta theme name")
		}
		key := "delta.syntax-theme"
		if port.Kind == "feature" {
			key = "delta.features"
		}
		settings[key] = port.Name
	}
	if len(settings) == 0 {
		return fmt.Errorf("delta custom file contains no native theme settings; provide syntax-theme, theme styles, or a named feature")
	}
	path := shared.NativePath("delta", env, filepath.Join(env.Home, ".gitconfig"))
	data, err := shared.Read(path, true)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Let Git parse and edit its own format, in an isolated temporary copy. No
	// global config command, include execution, or working-tree access occurs.
	file, err := os.CreateTemp(filepath.Dir(path), ".themeset-git-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	for key, value := range settings {
		if _, err := shared.Command(env, "git", "config", "--file", tmp, "--replace-all", key, value); err != nil {
			return err
		}
	}
	result, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	return shared.Atomic(path, result)
}
