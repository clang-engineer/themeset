package lazygit

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "lazygit", Binary: "lazygit", Remote: true, Resolve: resolve, Apply: apply}
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
		if !catalog.Verified("lazygit", port) {
			return fmt.Errorf("unregistered lazygit official port %q", port.Name)
		}
		data, err = shared.AcquireAsset(env, port.Source)
		if err == nil {
			if err = shared.ValidateLazyPort(data); err != nil {
				return fmt.Errorf("invalid %s lazygit port: %w", port.Name, err)
			}
		}
	} else {
		return fmt.Errorf("lazygit requires an available official port or a registered native theme file")
	}
	if err != nil {
		return err
	}
	return shared.ApplyLazy("lazygit", port, env, data)
}
