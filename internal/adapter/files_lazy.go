package adapter

import (
	"errors"
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesApplyLazy(id string) func(model.Port, model.Environment) error {
	return func(port model.Port, env model.Environment) error {
		data, err := filesPortData(id, port, env)
		if err != nil {
			return err
		}
		incoming, err := filesYAML(data)
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
		return filesEditYAML(filesPath(id, env), func(config map[string]any) error {
			gui, err := filesTable(config, "gui")
			if err != nil {
				return err
			}
			existing, err := filesTable(gui, "theme")
			if err != nil {
				return err
			}
			filesMerge(existing, theme)
			return nil
		})
	}
}

func filesValidateLazyPort(data []byte) error {
	config, err := filesYAML(data)
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
