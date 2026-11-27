package shared

import (
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
	"gopkg.in/yaml.v3"
)

func ApplyLazy(id string, port model.Port, env model.Environment, data []byte) error {
	incoming, err := YAML(data)
	if err != nil {
		return err
	}
	gui, _ := incoming["gui"].(map[string]any)
	if port.Kind == "official" && port.File == "" && id == "lazygit" && gui["theme"] == nil {
		gui = incoming
	}
	theme, ok := gui["theme"].(map[string]any)
	if !ok || len(theme) == 0 {
		return fmt.Errorf("%s custom file must contain a nonempty gui.theme mapping", id)
	}
	return EditYAML(Path(id, env), func(config *yaml.Node) error {
		gui, err := yamlPath(config, "gui")
		if err != nil {
			return err
		}
		existing, err := yamlPath(gui, "theme")
		if err != nil {
			return err
		}
		return MergeYAMLMap(existing, theme)
	})
}

func ValidateLazyPort(data []byte) error {
	config, err := YAML(data)
	if err != nil {
		return err
	}
	gui, _ := config["gui"].(map[string]any)
	theme, _ := gui["theme"].(map[string]any)
	if len(theme) == 0 {
		theme, _ = config["theme"].(map[string]any)
	}
	if len(theme) == 0 {
		return errors.New("missing lazygit theme mapping")
	}
	return nil
}
