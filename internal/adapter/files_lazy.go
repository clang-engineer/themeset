package adapter

import (
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
