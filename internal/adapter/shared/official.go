package shared

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/model"
)

// AcquireRosePinePort checks the registered recipe before fetching untrusted bytes.
// The caller validates its tool-native format before applying it.
func AcquireRosePinePort(id string, port model.Port, env model.Environment) ([]byte, error) {
	source, ok := RosePineSource(id)
	if !ok || port.Kind != "official" || port.Name != "rose-pine" || port.Source != source || port.File != "" {
		return nil, fmt.Errorf("unregistered Rosé Pine %s port", id)
	}
	data, err := AcquireAsset(env, source)
	if err != nil {
		return nil, fmt.Errorf("acquire Rosé Pine %s theme: %w", id, err)
	}
	return data, nil
}

// AcquireTokyoNightPort checks the registered recipe before fetching untrusted bytes.
func AcquireTokyoNightPort(id string, port model.Port, env model.Environment) ([]byte, error) {
	source, ok := TokyoNightSource(id)
	if !ok || id == "syntax" || id == "tmux" || port.Kind != "official" || port.File != "" || port.Source != source || id == "btop" && port.Name != "tokyonight_night" || id != "btop" && port.Name != "tokyo-night" {
		return nil, fmt.Errorf("unregistered Tokyo Night %s port", id)
	}
	data, err := AcquireAsset(env, source)
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night %s: %w", id, err)
	}
	return data, nil
}
