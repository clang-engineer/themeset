package app

import (
	"errors"
	"fmt"
	"sort"

	"github.com/clang-engineer/themeset/internal/model"
)

// selectTools validates the built-in adapter registry and returns the requested
// adapters in stable ID order.
func selectTools(ids []string, adapters []model.Adapter) ([]model.Adapter, error) {
	byID := make(map[string]model.Adapter, len(adapters))
	for _, adapter := range adapters {
		if adapter.ID == "" {
			return nil, errors.New("adapter registry contains an empty tool ID")
		}
		if adapter.Binary == "" || adapter.Resolve == nil || adapter.Apply == nil {
			return nil, fmt.Errorf("adapter %q is incomplete: binary, resolver, and apply function are required", adapter.ID)
		}
		if _, exists := byID[adapter.ID]; exists {
			return nil, fmt.Errorf("adapter registry contains duplicate tool ID %q", adapter.ID)
		}
		byID[adapter.ID] = adapter
	}
	if ids == nil {
		ids = make([]string, 0, len(adapters))
		for _, adapter := range adapters {
			ids = append(ids, adapter.ID)
		}
	}
	selected := make([]model.Adapter, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		adapter, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown tool %q (use 'list tools' to list IDs; Neovim's ID is neovim)", id)
		}
		if !seen[id] {
			selected = append(selected, adapter)
			seen[id] = true
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected, nil
}
