package yazi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "yazi", Binary: "yazi", Remote: true, Resolve: filesResolveYazi, Apply: filesApplyYazi}
}

func filesResolveYazi(spec model.ThemePort, env model.Environment) (model.Port, bool, error) {
	for _, name := range spec.Native {
		if !shared.Name(name) {
			continue
		}
		root := filesYaziFlavorRoot(name, env)
		installed, err := filesYaziFlavorInstalled(name, env)
		if err != nil {
			return model.Port{}, false, err
		}
		if !installed {
			continue
		}
		if spec.Official != nil {
			owned, err := filesYaziMatchesSource(root, spec.Official.Source)
			if err != nil {
				return model.Port{}, false, err
			}
			if owned {
				continue
			}
		}
		source := shared.YaziThemesSource
		if spec.Official != nil && spec.Official.Format == "yazi-flavor-bundle" {
			source = spec.Official.Source
		}
		return model.Port{Kind: "native", Name: name, File: filepath.Join(root, "flavor.toml"), Source: source}, true, nil
	}
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func filesApplyYazi(port model.Port, env model.Environment) error {
	if port.URL != "" {
		return filesApplyRemoteYazi(port, env)
	}
	if port.Kind == "official" && port.File == "" && port.URL == "" {
		if !catalog.Verified("yazi", port) {
			return fmt.Errorf("unregistered Yazi official port %q", port.Name)
		}
		switch port.Format {
		case "yazi-flavor-pair":
			return filesApplyTwoFileYazi(port, env)
		case "yazi-flavor-bundle":
			return filesApplyYaziBundle(port, env)
		case "yazi-direct":
			// The direct style file is merged below.
		default:
			return fmt.Errorf("unsupported Yazi official format %q", port.Format)
		}
	}
	if port.Kind == "native" {
		return filesApplyYaziFamily(port, env)
	}
	var incoming map[string]any
	var syntaxData []byte
	var themeData []byte
	data, err := filesPortData(port, env)
	if err != nil {
		return err
	}
	incoming, err = shared.TOML(data)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		return errors.New("Yazi theme file is empty")
	}
	if port.Kind == "official" && port.File == "" {
		if err := filesValidateYaziPort(data); err != nil {
			return err
		}
		themeData = data
		syntaxData, err = shared.AcquireAsset(env, port.SyntaxURL)
		if err != nil {
			return fmt.Errorf("acquire %s Yazi syntax: %w", port.Name, err)
		}
		var syntax struct {
			XMLName xml.Name `xml:"plist"`
		}
		if err := xml.Unmarshal(syntaxData, &syntax); err != nil || syntax.XMLName.Local != "plist" {
			return fmt.Errorf("invalid %s Yazi syntax: %v", port.Name, err)
		}
	}
	// theme.toml is dedicated to styles. Recursively merge supplied sections,
	// retaining unspecified style overrides; yazi.toml/keymap.toml are untouched.
	err = shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		if port.Kind == "official" && port.File == "" {
			return filesApplyOfficialYazi(config, incoming, syntaxData, port.Name, env)
		}
		shared.Merge(config, incoming)
		return nil
	})
	if err != nil || themeData == nil {
		return err
	}
	return shared.Atomic(filesYaziManagedStylesPath(port.Name, env), themeData)
}

func filesApplyYaziBundle(port model.Port, env model.Environment) error {
	if port.Kind != "official" || port.Format != "yazi-flavor-bundle" {
		return errors.New("unregistered bundled Yazi flavor")
	}
	return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		if err := filesInstallYaziBundle(port, env); err != nil {
			return err
		}
		flavor["dark"] = port.Name
		flavor["light"] = port.Name
		delete(flavor, "use")
		return nil
	})
}

func filesValidateYaziPort(data []byte) error {
	config, err := shared.TOML(data)
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return errors.New("Yazi direct style has no [mgr] styles")
	}
	return nil
}

func filesPortData(port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" || port.URL != "" {
		return shared.Custom(port, env)
	}
	if port.Kind == "official" {
		return shared.AcquireAsset(env, port.Source)
	}
	return nil, fmt.Errorf("yazi requires an available official port or a registered native theme file")
}

var filesYaziColor = regexp.MustCompile(`^(#[[:xdigit:]]{3}|#[[:xdigit:]]{6}|reset|black|red|green|yellow|blue|magenta|cyan|white|darkgray|gray|lightred|lightgreen|lightyellow|lightblue|lightmagenta|lightcyan)$`)

// Remote theme files are untrusted: accept UI style tables only. In particular,
// never import flavor selectors, manager settings, plugins or supplied syntax paths.
func filesApplyRemoteYazi(port model.Port, env model.Environment) error {
	if port.Kind != "custom" || port.File != "" || !shared.Name(port.Name) || port.SyntaxURL == "" {
		return errors.New("remote Yazi theme requires a safe name, theme URL and syntax_url")
	}
	data, err := shared.Custom(port, env)
	if err != nil {
		return err
	}
	incoming, err := shared.TOML(data)
	if err != nil {
		return err
	}
	styles := make(map[string]any)
	for _, section := range []string{"app", "mgr", "tabs", "mode", "indicator", "status", "input", "pick", "confirm", "spot", "completion", "tasks", "which", "help", "notify"} {
		table, ok := incoming[section].(map[string]any)
		if !ok {
			continue
		}
		selected := make(map[string]any)
		for key, value := range table {
			style, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if !filesYaziStyle(style) {
				return fmt.Errorf("invalid Yazi %s.%s style", section, key)
			}
			for attribute, color := range style {
				switch attribute {
				case "fg", "bg":
					value, ok := color.(string)
					if !ok || value == "" || !filesYaziColor.MatchString(value) {
						return fmt.Errorf("invalid Yazi %s.%s.%s color", section, key, attribute)
					}
				default:
					if _, ok := color.(bool); !ok {
						return fmt.Errorf("invalid Yazi %s.%s.%s flag", section, key, attribute)
					}
				}
			}
			selected[key] = style
		}
		if len(selected) > 0 {
			styles[section] = selected
		}
	}
	if len(styles) == 0 {
		return errors.New("remote Yazi theme has no UI styles")
	}
	syntaxData, err := shared.AcquireAsset(env, port.SyntaxURL)
	if err != nil {
		return fmt.Errorf("acquire remote Yazi syntax: %w", err)
	}
	var syntax struct {
		XMLName xml.Name  `xml:"plist"`
		Dict    *struct{} `xml:"dict"`
	}
	if err := xml.Unmarshal(syntaxData, &syntax); err != nil || syntax.XMLName.Local != "plist" || syntax.Dict == nil {
		return fmt.Errorf("invalid remote Yazi syntax XML: %v", err)
	}
	snapshot, err := toml.Marshal(styles)
	if err != nil {
		return fmt.Errorf("encode managed Yazi styles: %w", err)
	}
	directory := filepath.Dir(shared.Path("yazi", env))
	syntaxPath := filepath.Join(directory, "themeset-custom-"+port.Name+".tmTheme")
	err = shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		flavor["dark"] = ""
		flavor["light"] = ""
		delete(flavor, "use")
		for section, value := range styles {
			target, err := shared.Table(config, section)
			if err != nil {
				return err
			}
			shared.Merge(target, value.(map[string]any))
		}
		mgr, err := shared.Table(config, "mgr")
		if err != nil {
			return err
		}
		if err := shared.Atomic(syntaxPath, syntaxData); err != nil {
			return err
		}
		mgr["syntect_theme"] = syntaxPath
		return nil
	})
	if err != nil {
		return err
	}
	return shared.Atomic(filepath.Join(directory, ".themeset-custom-"+port.Name+"-styles.toml"), snapshot)
}
