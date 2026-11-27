package app

import (
	"fmt"
	"time"

	"github.com/clang-engineer/themeset/internal/model"
)

// apply executes resolved ports and records each tool's outcome independently.
func apply(result *Result, env model.Environment, adapters []model.Adapter) {
	byID := make(map[string]model.Adapter, len(adapters))
	for _, adapter := range adapters {
		byID[adapter.ID] = adapter
	}
	for i := range result.Tools {
		item := &result.Tools[i]
		if item.Port == nil {
			continue
		}
		if err := byID[item.Tool].Apply(*item.Port, env); err != nil {
			item.Status, item.Outcome = "error", "failed"
			message := fmt.Sprintf("apply theme: %v", err)
			if item.Error != "" {
				message = item.Error + "; " + message
			}
			item.Error = message
		} else {
			item.Outcome = "applied"
		}
	}
	result.AppliedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func failed(result Result) bool {
	for _, item := range result.Tools {
		if item.Error != "" {
			return true
		}
	}
	return false
}
