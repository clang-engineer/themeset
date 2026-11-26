package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/clang-engineer/themeset/internal/model"
)

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

const rosePineDeltaSource = "https://raw.githubusercontent.com/rose-pine/delta/38d58cbd01d9d619e2b3d63cdb63005451c8238c/dist/rose-pine.gitconfig"
const rosePineDeltaDigest = "914b6b33b3d2b1ce70b872a5c7affef9b3e0f15977887472fa3bb07dd307fe15"
const rosePineDeltaLicense = "https://raw.githubusercontent.com/rose-pine/delta/38d58cbd01d9d619e2b3d63cdb63005451c8238c/LICENSE"
const rosePineDeltaLicenseDigest = "638a6cb33a2b304656cd8e47e53d5eba925adbe79bac9eb89dae099ae073263c"

func nativeResolveDelta(theme string, env model.Environment) (model.Port, bool, error) {
	if theme == "rose-pine" {
		return model.Port{Kind: "official", Name: theme, Source: rosePineDeltaSource}, true, nil
	}
	return nativeListedResolver("delta", "--list-syntax-themes")(theme, env)
}

// Import only the author's main Delta feature. The same upstream file contains
// unrelated Moon/Dawn variants and default dark/light selectors; never import
// those into the user's global Git configuration.
func nativeRosePineDeltaMain(data []byte) ([]byte, error) {
	text := string(data)
	const next = "\n[delta \"rose-pine-moon\"]"
	main, _, ok := strings.Cut(text, next)
	if !ok || !strings.HasPrefix(main, "[delta \"rose-pine\"]\n") || !strings.Contains(main, "\n\tfile-style = ") {
		return nil, fmt.Errorf("Rosé Pine Delta feature missing from official asset")
	}
	return []byte(main + "\n"), nil
}

func nativeApplyRosePineDelta(port model.Port, env model.Environment) error {
	if port.Kind != "official" || port.Name != "rose-pine" || port.Source != rosePineDeltaSource || port.File != "" {
		return fmt.Errorf("unregistered Rosé Pine Delta feature")
	}
	data, err := filesAcquireAsset(env, rosePineDeltaSource, rosePineDeltaDigest)
	if err != nil {
		return err
	}
	main, err := nativeRosePineDeltaMain(data)
	if err != nil {
		return err
	}
	if _, err := filesAcquireAsset(env, rosePineDeltaLicense, rosePineDeltaLicenseDigest); err != nil {
		return err
	}
	file, err := os.CreateTemp(env.Home, ".themeset-rose-pine-delta-*.gitconfig")
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
	return nativeApplyDelta(model.Port{Kind: "feature", Name: "rose-pine", File: file.Name()}, env)
}

func nativeApplyDelta(port model.Port, env model.Environment) error {
	if port.Kind == "official" && port.Name == "rose-pine" {
		return nativeApplyRosePineDelta(port, env)
	}
	settings := map[string]string{}
	if port.File != "" {
		source := nativeExpand(port.File, env)
		info, err := os.Stat(source)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("delta custom config must be a regular gitconfig file")
		}
		out, err := nativeCommand(env, "git", "config", "--file", source, "--no-includes", "--null", "--list")
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
	path := nativePath("delta", env, filepath.Join(env.Home, ".gitconfig"))
	data, err := nativeRead(path)
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
		if _, err := nativeCommand(env, "git", "config", "--file", tmp, "--replace-all", key, value); err != nil {
			return err
		}
	}
	result, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	return nativeWrite(path, result)
}
