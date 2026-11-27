package television

import (
	"errors"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "television", Binary: "tv", Resolve: resolve, Apply: apply}
}

func resolve(spec model.ThemePort, _ model.Environment) (model.Port, bool, error) {
	if len(spec.Native) == 0 {
		return model.Port{}, false, nil
	}
	return model.Port{Kind: "native", Name: spec.Native[0]}, true, nil
}

func apply(port model.Port, env model.Environment) error {
	if port.Kind != "native" || port.Name == "" || !catalog.KnownNative("television", port.Name) {
		return errors.New("unknown native television theme")
	}
	return shared.EditTOML(shared.Path("television", env), func(config map[string]any) error {
		ui, err := shared.Table(config, "ui")
		if err != nil {
			return err
		}
		ui["theme"] = port.Name
		return nil
	})
}
