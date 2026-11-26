package lazydocker

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "lazydocker", Binary: "lazydocker", Resolve: resolve, Apply: apply}
}

func resolve(name string, _ model.Environment) (model.Port, bool, error) {
	port, ok := shared.OfficialPort("lazydocker", name)
	return port, ok, nil
}

func apply(port model.Port, env model.Environment) error {
	var data []byte
	var err error
	if port.File != "" {
		data, err = shared.Custom(port, env)
	} else if port.Kind == "official" {
		if registered, ok := shared.OfficialPort("lazydocker", port.Name); ok && registered.Source == port.Source {
			data, err = shared.AcquireAsset(env, port.Source)
		} else {
			return fmt.Errorf("lazydocker requires an available official port or a registered native theme file")
		}
	} else {
		return fmt.Errorf("lazydocker requires an available official port or a registered native theme file")
	}
	if err != nil {
		return err
	}
	return shared.ApplyLazy("lazydocker", port, env, data)
}
