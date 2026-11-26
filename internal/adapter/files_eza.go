package adapter

import (
	"errors"

	"github.com/clang-engineer/themeset/internal/model"
)

func filesApplyEza(port model.Port, env model.Environment) error {
	data, err := filesPortData("eza", port, env)
	if err != nil {
		return err
	}
	incoming, err := filesYAML(data)
	if err != nil {
		return err
	}
	if len(incoming) == 0 {
		return errors.New("eza theme file is empty")
	}
	return filesEditYAML(filesPath("eza", env), func(config map[string]any) error {
		filesMerge(config, incoming)
		return nil
	})
}
