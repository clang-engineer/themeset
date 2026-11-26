package yazi

import (
	"encoding/xml"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

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

// Activate only a previously detected native flavor. Explicit custom ports
// must stay on the caller's existing custom path and never enter this function.
func filesApplyYaziFamily(port model.Port, env model.Environment) error {
	if !shared.Name(port.Name) || port.Kind != "native" || port.File != filepath.Join(filesYaziFlavorRoot(port.Name, env), "flavor.toml") {
		return errors.New("Yazi native flavor must be an installed complete flavor")
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
