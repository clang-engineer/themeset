package yazi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

// The Yazi-maintained flavors repository ships neither Gruvbox nor Nord nor
// Moonfly. Its README links to independent community ports for the first two;
// the Moonfly author's extras do not include a Yazi flavor. Consequently none
// of these names has a trusted automatic acquisition recipe. An independently
// installed complete flavor is nevertheless a native user-provided port.
func filesYaziFamilyName(name string) bool {
	switch name {
	case "gruvbox", "nord", "moonfly":
		return true
	}
	return false
}

func filesYaziFlavorRoot(name string, env model.Environment) string {
	return filepath.Join(filepath.Dir(shared.Path("yazi", env)), "flavors", name+".yazi")
}

// A native flavor needs actual runtime colors and a readable syntax theme.
// No license hash is required for a flavor installed independently by its user.
func filesYaziFlavorInstalled(name string, env model.Environment) (bool, error) {
	root := filesYaziFlavorRoot(name, env)
	data, err := shared.Read(filepath.Join(root, "flavor.toml"), true)
	if err != nil || len(data) == 0 {
		return false, err
	}
	config, err := shared.TOML(data)
	if err != nil {
		return false, fmt.Errorf("parse installed %s flavor: %w", name, err)
	}
	mgr, ok := config["mgr"].(map[string]any)
	if !ok || len(mgr) == 0 {
		return false, fmt.Errorf("installed %s flavor requires a nonempty [mgr] table", name)
	}
	data, err = shared.Read(filepath.Join(root, "tmtheme.xml"), true)
	if err != nil || len(data) == 0 {
		return false, err
	}
	var theme struct {
		XMLName xml.Name `xml:"plist"`
	}
	if err := xml.Unmarshal(data, &theme); err != nil {
		return false, fmt.Errorf("parse installed %s syntax theme: %w", name, err)
	}
	return true, nil
}

func filesResolveYaziFamily(name string, env model.Environment) (model.Port, bool, error) {
	if !filesYaziFamilyName(name) {
		return model.Port{}, false, nil
	}
	installed, err := filesYaziFlavorInstalled(name, env)
	if err != nil || !installed {
		return model.Port{}, false, err
	}
	return model.Port{
		Kind: "native", Name: name,
		File:   filepath.Join(filesYaziFlavorRoot(name, env), "flavor.toml"),
		Source: filesYaziSource,
	}, true, nil
}

// Activate only a previously detected native flavor. Explicit custom ports
// must stay on the caller's existing custom path and never enter this function.
func filesApplyYaziFamily(port model.Port, env model.Environment) error {
	if !filesYaziFamilyName(port.Name) || port.Kind != "native" || port.File != filepath.Join(filesYaziFlavorRoot(port.Name, env), "flavor.toml") {
		return errors.New("Yazi family requires a registered native flavor")
	}
	installed, err := filesYaziFlavorInstalled(port.Name, env)
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("Yazi flavor %q is not installed completely", port.Name)
	}
	return shared.EditTOML(shared.Path("yazi", env), func(config map[string]any) error {
		flavor, err := shared.Table(config, "flavor")
		if err != nil {
			return err
		}
		if err := filesRemovePreviousYaziStyles(config, env); err != nil {
			return err
		}
		flavor["dark"] = port.Name
		flavor["light"] = port.Name
		delete(flavor, "use")
		return nil
	})
}
