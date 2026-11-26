package lazygit

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/adapter/shared"
	"github.com/clang-engineer/themeset/internal/model"
)

func Adapter() model.Adapter {
	return model.Adapter{ID: "lazygit", Binary: "lazygit", Remote: true, Resolve: resolve, Apply: apply}
}

func resolve(name string, env model.Environment) (model.Port, bool, error) {
	if port, found, err := shared.ResolveTokyoNightPort("lazygit", name, env); found || err != nil {
		return port, found, err
	}
	if port, found, err := shared.ResolveRosePinePort("lazygit", name, env); found || err != nil {
		return port, found, err
	}
	port, ok := shared.OfficialPort("lazygit", name)
	return port, ok, nil
}

func apply(port model.Port, env model.Environment) error {
	var data []byte
	var err error
	if port.File != "" || port.URL != "" {
		data, err = shared.Custom(port, env)
	} else if port.Kind == "official" {
		switch port.Name {
		case "tokyo-night":
			data, err = shared.AcquireTokyoNightPort("lazygit", port, env)
		case "rose-pine":
			data, err = shared.AcquireRosePinePort("lazygit", port, env)
		default:
			if registered, ok := shared.OfficialPort("lazygit", port.Name); ok && registered.Source == port.Source {
				data, err = shared.AcquireAsset(env, port.Source)
			} else {
				return fmt.Errorf("lazygit requires an available official port or a registered native theme file")
			}
		}
		if err == nil && (port.Name == "tokyo-night" || port.Name == "rose-pine") {
			err = shared.ValidateLazyPort(data)
			if err != nil {
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
