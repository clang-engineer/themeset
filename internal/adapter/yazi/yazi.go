package yazi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
	"github.com/pelletier/go-toml/v2"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "yazi", Binary: "yazi", Remote: true, Resolve: filesResolveYazi, Apply: filesApplyYazi}
}

func filesResolveYazi(name string, env model.Environment) (model.Port, bool, error) {
	if name == "dracula" {
		return filesResolveYaziDracula(env)
	}
	if port, found, err := filesResolveTokyoNightYazi(name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := filesResolveRosePineYazi(name, env); found || err != nil {
		return port, found, err
	}
	if port, ok := shared.OfficialPort("yazi", name); ok {
		return port, true, nil
	}
	if filesYaziFamilyName(name) {
		return filesResolveYaziFamily(name, env)
	}
	if !shared.Name(name) {
		return model.Port{}, false, nil
	}
	root := filepath.Join(filepath.Dir(shared.Path("yazi", env)), "flavors", name+".yazi")
	data, err := shared.Read(filepath.Join(root, "flavor.toml"), true)
	if err != nil {
		return model.Port{}, false, err
	}
	if len(data) == 0 {
		return model.Port{}, false, nil
	}
	if _, err := shared.TOML(data); err != nil {
		return model.Port{}, false, err
	}
	if _, err := shared.Read(filepath.Join(root, "tmtheme.xml"), false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return model.Port{}, false, nil
		}
		return model.Port{}, false, err
	}
	return model.Port{Kind: "native", Name: name, File: filepath.Join(root, "flavor.toml"), Source: shared.YaziThemesSource}, true, nil
}

func filesApplyYazi(port model.Port, env model.Environment) error {
	if port.URL != "" {
		return filesApplyRemoteYazi(port, env)
	}
	if port.Name == "tokyo-night" && (port.Kind == "official" && port.File == "" || port.Kind == "native") {
		return filesApplyTokyoNightYazi(port, env)
	}
	if port.Name == "rose-pine" && (port.Kind == "official" && port.File == "" || port.Kind == "native") {
		return filesApplyRosePineYazi(port, env)
	}
	if port.Kind == "native" && filesYaziFamilyName(port.Name) {
		return filesApplyYaziFamily(port, env)
	}
	if port.Name == "dracula" && (port.Kind == "native" || port.Kind == "official" && port.File == "") {
		return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
			flavor, err := shared.Table(config, "flavor")
			if err != nil {
				return err
			}
			if err := filesRemovePreviousYaziStyles(config, env); err != nil {
				return err
			}
			if err := filesInstallYaziDracula(env); err != nil {
				return err
			}
			flavor["dark"] = "dracula"
			flavor["light"] = "dracula"
			delete(flavor, "use")
			return nil
		})
	}
	var incoming map[string]any
	var syntaxData []byte
	var themeData []byte
	if port.Kind == "native" {
		if _, ok, err := filesResolveYazi(port.Name, env); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Yazi flavor %q is not installed", port.Name)
		}
		incoming = map[string]any{"flavor": map[string]any{"dark": port.Name, "light": port.Name}}
	} else {
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
			syntaxData, err = shared.AcquireAsset(env, shared.YaziCatppuccinSyntaxSource)
			if err != nil {
				return fmt.Errorf("acquire Catppuccin Yazi syntax: %w", err)
			}
			var syntax struct {
				XMLName xml.Name `xml:"plist"`
			}
			if err := xml.Unmarshal(syntaxData, &syntax); err != nil || syntax.XMLName.Local != "plist" {
				return fmt.Errorf("invalid Catppuccin Yazi syntax: %v", err)
			}
		}
	}
	// theme.toml is dedicated to styles. Recursively merge supplied sections,
	// retaining unspecified style overrides; yazi.toml/keymap.toml are untouched.
	err := shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		if port.Kind == "official" && port.File == "" {
			return filesApplyOfficialYazi(config, incoming, syntaxData, env)
		}
		if port.Kind == "native" {
			if err := filesRemovePreviousYaziStyles(config, env); err != nil {
				return err
			}
		}
		shared.Merge(config, incoming)
		return nil
	})
	if err != nil || themeData == nil {
		return err
	}
	return shared.Atomic(filesYaziManagedStylesPath(env), themeData)
}

func filesValidateYaziPort(data []byte) error {
	config, err := shared.TOML(data)
	if err != nil {
		return err
	}
	mgr, _ := config["mgr"].(map[string]any)
	if len(mgr) == 0 {
		return errors.New("Tokyo Night Yazi flavor has no [mgr] styles")
	}
	return nil
}

func filesPortData(port model.Port, env model.Environment) ([]byte, error) {
	if port.File != "" || port.URL != "" {
		return shared.Custom(port, env)
	}
	if port.Kind == "official" {
		if registered, ok := shared.OfficialPort("yazi", port.Name); ok && registered.Source == port.Source {
			return shared.AcquireAsset(env, port.Source)
		}
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
