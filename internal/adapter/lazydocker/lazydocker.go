package lazydocker

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "lazydocker", Binary: "lazydocker", Remote: true, Resolve: resolve, Apply: apply}
}

func resolve(spec model.ThemePort, _ model.Environment) (model.Port, bool, error) {
	if spec.Official != nil {
		return *spec.Official, true, nil
	}
	return model.Port{}, false, nil
}

func apply(port model.Port, env model.Environment) error {
	var data []byte
	var err error
	if port.File != "" || port.URL != "" {
		data, err = shared.Custom(port, env)
	} else if port.Kind == "official" {
		if !catalog.Verified("lazydocker", port) {
			return fmt.Errorf("unregistered lazydocker official port %q", port.Name)
		}
		data, err = shared.AcquireAsset(env, port.Source)
	} else {
		return fmt.Errorf("lazydocker requires an available official port or a registered native theme file")
	}
	if err != nil {
		return err
	}
	return shared.ApplyLazy("lazydocker", port, env, data)
}
