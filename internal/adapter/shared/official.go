package shared

import (
	"fmt"

	"github.com/clang-engineer/themeset/internal/catalog"
	"github.com/clang-engineer/themeset/internal/model"
)

// AcquireRosePinePort checks the registered recipe before fetching untrusted bytes.
// The caller validates its tool-native format before applying it.
func AcquireRosePinePort(id string, port model.Port, env model.Environment) ([]byte, error) {
	expected := catalog.Port("rose-pine", id).Official
	if expected == nil || port.Kind != "official" || port.Name != expected.Name || port.Source != expected.Source || port.File != "" || port.URL != "" {
		return nil, fmt.Errorf("unregistered Rosé Pine %s port", id)
	}
	data, err := AcquireAsset(env, expected.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire Rosé Pine %s theme: %w", id, err)
	}
	return data, nil
}

// AcquireTokyoNightPort checks the registered recipe before fetching untrusted bytes.
func AcquireTokyoNightPort(id string, port model.Port, env model.Environment) ([]byte, error) {
	expected := catalog.Port("tokyo-night", id).Official
	if expected == nil || id == "syntax" || id == "tmux" || port.Kind != "official" || port.File != "" || port.URL != "" || port.Source != expected.Source || port.Name != expected.Name {
		return nil, fmt.Errorf("unregistered Tokyo Night %s port", id)
	}
	data, err := AcquireAsset(env, expected.Source)
	if err != nil {
		return nil, fmt.Errorf("acquire Tokyo Night %s: %w", id, err)
	}
	return data, nil
}
